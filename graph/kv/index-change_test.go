package kv

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/bolt"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// predicateIndex is the predicate-leading index the tests build and drop.
var predicateIndex = QuadIndex{Dirs: []quad.Direction{quad.Predicate, quad.Label, quad.Subject}}

// withPredicateIndex is the default layout plus predicateIndex.
var withPredicateIndex = slices.Concat(DefaultQuadIndexes, []QuadIndex{predicateIndex})

// changeStore is one store under test with its database.
type changeStore struct {
	db hkv.KV
	qs *QuadStore
}

// openChangeStore opens a store on a new database with the options.
func openChangeStore(t *testing.T, opt graph.Options) *changeStore {
	t.Helper()
	db := btree.New()
	t.Cleanup(func() { db.Close() })
	return &changeStore{db: db, qs: reopenChangeStore(t, db, opt)}
}

// reopenChangeStore opens a new handle on db.
func reopenChangeStore(t *testing.T, db hkv.KV, opt graph.Options) *QuadStore {
	t.Helper()
	g, err := New(t.Context(), db, opt)
	require.NoError(t, err)
	return g.(*QuadStore)
}

// changeWorkload applies the same deterministic writes and deletes to stores.
type changeWorkload struct {
	rng  *rand.Rand
	live []quad.Quad
}

// newChangeWorkload seeds a workload.
func newChangeWorkload() *changeWorkload {
	return &changeWorkload{rng: rand.New(rand.NewPCG(1, 2))}
}

// round picks adds and deletes and applies them to every store, each in one
// transaction.
func (w *changeWorkload) round(t *testing.T, adds, dels int, stores ...*QuadStore) {
	t.Helper()

	// Choose new links over a small vocabulary, so postings are shared.
	var deltas []graph.Delta
	for range adds {
		label := ""
		if w.rng.IntN(2) == 0 {
			label = fmt.Sprintf("l%d", w.rng.IntN(3))
		}
		q := quad.MakeIRI(
			fmt.Sprintf("s%d", w.rng.IntN(20)),
			fmt.Sprintf("p%d", w.rng.IntN(5)),
			fmt.Sprintf("o%d", w.rng.IntN(20)),
			label,
		)
		if slices.Contains(w.live, q) || slices.ContainsFunc(deltas, func(d graph.Delta) bool { return d.Quad == q }) {
			continue
		}
		deltas = append(deltas, graph.Delta{Quad: q, Action: graph.Add})
	}

	// Choose links written earlier to delete.
	for range min(dels, len(w.live)) {
		at := w.rng.IntN(len(w.live))
		deltas = append(deltas, graph.Delta{Quad: w.live[at], Action: graph.Delete})
		w.live = slices.Delete(w.live, at, at+1)
	}
	for _, d := range deltas {
		if d.Action == graph.Add {
			w.live = append(w.live, d.Quad)
		}
	}

	// Apply the same transaction to every store.
	for _, qs := range stores {
		require.NoError(t, qs.ApplyDeltas(t.Context(), deltas, graph.IgnoreOpts{}))
	}
}

// changeLookups are lookups by predicate, predicate and label, and the
// directions the default layout serves.
var changeLookups = func() []quad.Quad {
	var out []quad.Quad
	for p := range 5 {
		out = append(out, quad.MakeIRI("", fmt.Sprintf("p%d", p), "", ""))
		for l := range 3 {
			out = append(out, quad.MakeIRI("", fmt.Sprintf("p%d", p), "", fmt.Sprintf("l%d", l)))
		}
	}
	return append(out,
		quad.MakeIRI("s3", "", "", ""),
		quad.MakeIRI("s3", "p1", "", ""),
		quad.MakeIRI("", "", "o4", ""),
		quad.MakeIRI("", "p2", "o4", ""),
	)
}()

// requireSameLookups requires every lookup to return the same quads, in the
// same order, from both stores.
func requireSameLookups(t *testing.T, want, got *QuadStore) {
	// Run the lookups on each store and compare.
	t.Helper()
	wantRes, err := want.CollectFilteredQuadsBatch(t.Context(), changeLookups, 0)
	require.NoError(t, err)
	gotRes, err := got.CollectFilteredQuadsBatch(t.Context(), changeLookups, 0)
	require.NoError(t, err)
	require.Equal(t, wantRes, gotRes)
}

// dumpStore returns every key and value in db.
func dumpStore(t *testing.T, db hkv.KV) map[string]string {
	t.Helper()
	out := make(map[string]string)
	require.NoError(t, hkv.View(t.Context(), db, func(tx hkv.Tx) error {
		it := tx.Scan(t.Context())
		defer it.Close()
		for it.Next(t.Context()) {
			out[fmt.Sprintf("%q", it.Key())] = string(it.Val())
		}
		return it.Err()
	}))
	return out
}

// TestIndexChangeBuild builds the predicate index online under writes and
// deletes. At every step lookups match a store created with the index, two
// stores stepped alike hold the same bytes, and the finished store holds the
// same bytes as the created one.
func TestIndexChangeBuild(t *testing.T) {
	// Open a store to build on, a twin stepped alike, and one created with
	// the index.
	ctx := t.Context()
	built := openChangeStore(t, nil)
	twin := openChangeStore(t, nil)
	created := openChangeStore(t, graph.Options{OptQuadIndexes: withPredicateIndex})

	// Load the stores, then begin the build on two of them.
	w := newChangeWorkload()
	w.round(t, 120, 0, built.qs, twin.qs, created.qs)
	w.round(t, 10, 30, built.qs, twin.qs, created.qs)
	require.NoError(t, built.qs.BeginIndexChange(ctx, withPredicateIndex))
	require.NoError(t, twin.qs.BeginIndexChange(ctx, withPredicateIndex))
	require.Equal(t, QuadIndexState_BUILDING, built.qs.Layout()[2].State)

	// Step under concurrent writes and deletes until the build finishes.
	var steps int
	for done := false; !done; steps++ {
		require.Less(t, steps, 1000, "build never finished")
		w.round(t, 3, 2, built.qs, twin.qs, created.qs)
		var err error
		done, err = built.qs.StepIndexChange(ctx, 7)
		require.NoError(t, err)
		twinDone, err := twin.qs.StepIndexChange(ctx, 7)
		require.NoError(t, err)
		require.Equal(t, done, twinDone)
		requireSameLookups(t, created.qs, built.qs)
		require.Equal(t, dumpStore(t, twin.db), dumpStore(t, built.db))
	}
	require.Greater(t, steps, 5, "the build should take several steps")

	// The finished index serves the lookup, and the store matches the created one.
	require.Equal(t, withPredicateIndex, built.qs.Layout())
	ind, ok := built.qs.postingScanIndex(map[quad.Direction]uint64{quad.Predicate: 1, quad.Label: 2})
	require.True(t, ok)
	require.Equal(t, predicateIndex, ind)
	require.Equal(t, dumpStore(t, created.db), dumpStore(t, built.db))
	w.round(t, 5, 5, built.qs, created.qs)
	requireSameLookups(t, created.qs, built.qs)
	require.Equal(t, dumpStore(t, created.db), dumpStore(t, built.db))

	// A further step does nothing.
	done, err := built.qs.StepIndexChange(ctx, 7)
	require.NoError(t, err)
	require.True(t, done)
}

// TestIndexChangeResume reopens a store in the middle of a build, from the
// stored layout and from a trusted copy, and finishes it.
func TestIndexChangeResume(t *testing.T) {
	// Use bolt, whose failed commits write nothing.
	ctx := t.Context()
	boltDB, err := bolt.Open(filepath.Join(t.TempDir(), "graph.db"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { boltDB.Close() })
	built := &changeStore{db: boltDB, qs: reopenChangeStore(t, boltDB, nil)}
	created := openChangeStore(t, graph.Options{OptQuadIndexes: withPredicateIndex})
	w := newChangeWorkload()
	w.round(t, 120, 20, built.qs, created.qs)

	// Begin, step twice, and reopen from the stored layout.
	require.NoError(t, built.qs.BeginIndexChange(ctx, withPredicateIndex))
	for range 2 {
		done, err := built.qs.StepIndexChange(ctx, 5)
		require.NoError(t, err)
		require.False(t, done)
	}
	layout := built.qs.Layout()
	built.qs = reopenChangeStore(t, built.db, nil)
	require.Equal(t, layout, built.qs.Layout())

	// A step whose commit fails leaves the store and layout where they were.
	before := dumpStore(t, built.db)
	failing := reopenChangeStore(t, failCommitKV{built.db}, nil)
	_, err = failing.StepIndexChange(ctx, 5)
	require.ErrorIs(t, err, errCommitFailed)
	require.Equal(t, before, dumpStore(t, built.db))
	require.Equal(t, layout, failing.Layout())

	// Reopen from a trusted copy, write, and finish.
	built.qs = reopenChangeStore(t, built.db, graph.Options{OptTrustQuadIndexes: layout})
	w.round(t, 4, 4, built.qs, created.qs)
	for done := false; !done; {
		done, err = built.qs.StepIndexChange(ctx, 5)
		require.NoError(t, err)
		requireSameLookups(t, created.qs, built.qs)
	}
	require.Equal(t, dumpStore(t, created.db), dumpStore(t, built.db))
}

// TestIndexChangeDrop drops the predicate index online under writes and
// deletes, ending with the bytes of a store that never had it.
func TestIndexChangeDrop(t *testing.T) {
	// Load a store created with the index and one without it.
	ctx := t.Context()
	dropped := openChangeStore(t, graph.Options{OptQuadIndexes: withPredicateIndex})
	plain := openChangeStore(t, nil)
	w := newChangeWorkload()
	w.round(t, 120, 20, dropped.qs, plain.qs)

	// Begin the drop; the index stops serving at once.
	require.NoError(t, dropped.qs.BeginIndexChange(ctx, DefaultQuadIndexes))
	require.Equal(t, QuadIndexState_DROPPING, dropped.qs.Layout()[2].State)
	ind, ok := dropped.qs.postingScanIndex(map[quad.Direction]uint64{quad.Predicate: 1, quad.Label: 2})
	require.True(t, ok)
	require.NotEqual(t, predicateIndex, ind)

	// Step under writes until the postings are gone.
	var steps int
	for done := false; !done; steps++ {
		require.Less(t, steps, 1000, "drop never finished")
		w.round(t, 3, 2, dropped.qs, plain.qs)
		var err error
		done, err = dropped.qs.StepIndexChange(ctx, 4)
		require.NoError(t, err)
		requireSameLookups(t, plain.qs, dropped.qs)
	}
	require.Greater(t, steps, 5, "the drop should take several steps")
	require.Equal(t, DefaultQuadIndexes, dropped.qs.Layout())
	require.Equal(t, dumpStore(t, plain.db), dumpStore(t, dropped.db))
}

// TestIndexChangeBuildAndDrop replaces one index with another in one change:
// the build finishes before the drop starts.
func TestIndexChangeBuildAndDrop(t *testing.T) {
	// Load a store with a label index and one created with the target.
	ctx := t.Context()
	labelIndex := QuadIndex{Dirs: []quad.Direction{quad.Label, quad.Predicate}}
	from := slices.Concat(DefaultQuadIndexes, []QuadIndex{labelIndex})
	changed := openChangeStore(t, graph.Options{OptQuadIndexes: from})
	created := openChangeStore(t, graph.Options{OptQuadIndexes: withPredicateIndex})
	w := newChangeWorkload()
	w.round(t, 60, 10, changed.qs, created.qs)

	// Build, then drop, then reach the created store's bytes.
	require.NoError(t, changed.qs.BeginIndexChange(ctx, withPredicateIndex))
	states := func() []QuadIndexState {
		var out []QuadIndexState
		for _, index := range changed.qs.Layout() {
			out = append(out, index.State)
		}
		return out
	}
	require.Equal(t, []QuadIndexState{QuadIndexState_COMPLETE, QuadIndexState_COMPLETE, QuadIndexState_BUILDING, QuadIndexState_DROPPING}, states())
	var sawDropOnly bool
	for done := false; !done; {
		w.round(t, 2, 1, changed.qs, created.qs)
		var err error
		done, err = changed.qs.StepIndexChange(ctx, 6)
		require.NoError(t, err)
		requireSameLookups(t, created.qs, changed.qs)
		if !done && slices.Equal(states(), []QuadIndexState{QuadIndexState_COMPLETE, QuadIndexState_COMPLETE, QuadIndexState_COMPLETE, QuadIndexState_DROPPING}) {
			sawDropOnly = true
		}
	}
	require.True(t, sawDropOnly, "the build should finish before the drop")
	require.Equal(t, dumpStore(t, created.db), dumpStore(t, changed.db))
}

// TestBeginIndexChangeRules covers the changes Begin refuses or ignores.
func TestBeginIndexChangeRules(t *testing.T) {
	// Load a store on the default layout.
	ctx := t.Context()
	s := openChangeStore(t, nil)
	newChangeWorkload().round(t, 20, 0, s.qs)
	other := QuadIndex{Dirs: []quad.Direction{quad.Label, quad.Subject}}

	// A target sharing no index, repeating a bucket, or not complete is refused.
	require.ErrorContains(t, s.qs.BeginIndexChange(ctx, []QuadIndex{predicateIndex}), "keeps no index")
	require.ErrorContains(t, s.qs.BeginIndexChange(ctx, slices.Concat(DefaultQuadIndexes, []QuadIndex{predicateIndex, {Dirs: predicateIndex.Dirs, Unique: true}})), "repeat posting bucket")
	require.ErrorContains(t, s.qs.BeginIndexChange(ctx, []QuadIndex{DefaultQuadIndexes[0], {Dirs: predicateIndex.Dirs, State: QuadIndexState_BUILDING}}), "request complete")

	// The layout already held is no change.
	before := dumpStore(t, s.db)
	require.NoError(t, s.qs.BeginIndexChange(ctx, DefaultQuadIndexes))
	require.Equal(t, before, dumpStore(t, s.db))

	// Beginning the change in progress again does nothing; another is refused.
	require.NoError(t, s.qs.BeginIndexChange(ctx, withPredicateIndex))
	before = dumpStore(t, s.db)
	require.NoError(t, s.qs.BeginIndexChange(ctx, withPredicateIndex))
	require.Equal(t, before, dumpStore(t, s.db))
	err := s.qs.BeginIndexChange(ctx, slices.Concat(DefaultQuadIndexes, []QuadIndex{other}))
	require.True(t, errors.Is(err, ErrIndexChangeInProgress), "%v", err)

	// A handle opened before the change refuses to step it.
	stale := reopenChangeStore(t, s.db, graph.Options{OptAssumeDefaultIdx: true})
	_, err = stale.StepIndexChange(ctx, 5)
	require.ErrorContains(t, err, "reopen the store")

	// A store whose layout says complete rejects a step size of zero.
	_, err = s.qs.StepIndexChange(ctx, 0)
	require.Error(t, err)
}

// TestTrustQuadIndexesRecordsOnFirstWrite opens an empty store with a trusted
// layout, which reads nothing, and its first write records the layout.
func TestTrustQuadIndexesRecordsOnFirstWrite(t *testing.T) {
	// Opening with the trusted layout writes nothing.
	trusted := openChangeStore(t, graph.Options{OptTrustQuadIndexes: withPredicateIndex})
	require.Empty(t, dumpStore(t, trusted.db))
	created := openChangeStore(t, graph.Options{OptQuadIndexes: withPredicateIndex})

	// Both stores hold the same bytes after the same writes.
	w := newChangeWorkload()
	w.round(t, 10, 0, trusted.qs, created.qs)
	w.round(t, 10, 3, trusted.qs, created.qs)
	require.Equal(t, dumpStore(t, created.db), dumpStore(t, trusted.db))
	require.Equal(t, withPredicateIndex, reopenChangeStore(t, trusted.db, nil).Layout())
}

// errCommitFailed is the error failCommitKV's commits return.
var errCommitFailed = errors.New("commit failed")

// failCommitKV is a database whose write transactions never commit.
type failCommitKV struct {
	hkv.KV
}

// Tx opens a transaction whose commit fails.
func (f failCommitKV) Tx(ctx context.Context, rw bool) (hkv.Tx, error) {
	tx, err := f.KV.Tx(ctx, rw)
	if err != nil {
		return nil, err
	}
	return failCommitTx{tx}, nil
}

// failCommitTx is a transaction whose commit fails without writing.
type failCommitTx struct {
	hkv.Tx
}

// Commit discards the transaction and fails.
func (f failCommitTx) Commit(ctx context.Context) error {
	f.Tx.Close()
	return errCommitFailed
}
