package kv

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/graph/proto"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

func TestRemoveQuadReclaimsLogAndIndexPostings(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	first := quad.MakeIRI("shared-subject", "shared-predicate", "first-object", "")
	second := quad.MakeIRI("shared-subject", "shared-predicate", "second-object", "")

	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: first, Action: graph.Add})
	firstPrim := onlyQuadPrimitive(t, ctx, db)
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: second, Action: graph.Add})
	secondPrim := newestQuadPrimitive(t, ctx, db)
	horizonBefore := readMetaInt(t, ctx, db, "horizon")

	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: first, Action: graph.Delete})

	require.Equal(t, horizonBefore, readMetaInt(t, ctx, db, "horizon"), "deleting a quad changed the monotonic horizon")
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		_, err := tx.Get(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(firstPrim.ID)))
		require.ErrorIs(t, err, hkv.ErrNotFound, "deleted quad log key %d remains", firstPrim.ID)

		_, err = tx.Get(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(firstPrim.Object)))
		require.ErrorIs(t, err, hkv.ErrNotFound, "unreferenced node log key %d remains", firstPrim.Object)

		for _, ind := range qs.indexes.all {
			key := ind.KeyFor(firstPrim)
			raw, err := tx.Get(ctx, key)
			if err == hkv.ErrNotFound {
				continue
			}
			require.NoError(t, err)
			ids, err := decodeIndex(raw)
			require.NoError(t, err)
			require.NotContains(t, ids, firstPrim.ID, "deleted quad id %d remains in index %v", firstPrim.ID, ind.Dirs)
		}

		shared, err := tx.Get(ctx, DefaultQuadIndexes[0].KeyFor(firstPrim))
		require.NoError(t, err)
		ids, err := decodeIndex(shared)
		require.NoError(t, err)
		require.Equal(t, []uint64{secondPrim.ID}, ids, "shared posting was not shortened to the live quad")

		_, err = tx.Get(ctx, DefaultQuadIndexes[1].KeyFor(firstPrim))
		require.ErrorIs(t, err, hkv.ErrNotFound, "empty posting key remains after deleting its only quad")
		return nil
	})
	require.NoError(t, err)
}

func TestRemoveHalfLargeStoreDropsKeyCountProportionally(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	const total = 200
	adds := make([]graph.Delta, total)
	for i := range adds {
		adds[i] = graph.Delta{
			Quad:   quad.MakeIRI(fmt.Sprintf("subject-%d", i), fmt.Sprintf("predicate-%d", i), fmt.Sprintf("object-%d", i), ""),
			Action: graph.Add,
		}
	}
	applyReclaimTestDeltas(t, ctx, qs, adds...)
	before := countKVKeys(t, ctx, db)

	deletes := make([]graph.Delta, total/2)
	for i := range deletes {
		deletes[i] = graph.Delta{Quad: adds[i].Quad, Action: graph.Delete}
	}
	applyReclaimTestDeltas(t, ctx, qs, deletes...)
	after := countKVKeys(t, ctx, db)

	require.LessOrEqual(t, after*100, before*55, "removing half the quads did not reclaim a proportional number of keys: before=%d after=%d", before, after)
	stats, err := qs.Stats(ctx, false)
	require.NoError(t, err)
	require.EqualValues(t, total/2, stats.Quads.Value)
}

// TestRemoveQuadDropsLogKeysAndPostingEntries counts the two things a delete
// is supposed to reclaim, rather than probing individual keys: entries in the
// log bucket, and ids listed across every index posting list.
func TestRemoveQuadDropsLogKeysAndPostingEntries(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	const total = 20
	adds := make([]graph.Delta, total)
	for i := range adds {
		adds[i] = graph.Delta{
			Quad:   quad.MakeIRI(fmt.Sprintf("subject-%d", i), "shared-predicate", fmt.Sprintf("object-%d", i), ""),
			Action: graph.Add,
		}
	}
	applyReclaimTestDeltas(t, ctx, qs, adds...)
	logBefore, postingsBefore := countLogAndPostingEntries(t, ctx, db)

	deletes := make([]graph.Delta, total/2)
	for i := range deletes {
		deletes[i] = graph.Delta{Quad: adds[i].Quad, Action: graph.Delete}
	}
	applyReclaimTestDeltas(t, ctx, qs, deletes...)
	logAfter, postingsAfter := countLogAndPostingEntries(t, ctx, db)

	// Each removed quad drops its own log entry plus the subject and object
	// nodes it was the sole reference for. The shared predicate stays live.
	require.Equal(t, logBefore-3*len(deletes), logAfter,
		"log entries: before=%d after=%d", logBefore, logAfter)
	// Each removed quad is listed once per index.
	require.Equal(t, postingsBefore-len(qs.indexes.all)*len(deletes), postingsAfter,
		"posting entries: before=%d after=%d", postingsBefore, postingsAfter)
}

func TestLegacyTombstonesRemainAbsent(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	q := quad.MakeIRI("legacy-subject", "legacy-predicate", "legacy-object", "")
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Add})
	prim := onlyQuadPrimitive(t, ctx, db)

	err := hkv.Update(ctx, db, func(tx hkv.Tx) error {
		quadTombstone := prim.CloneVT()
		quadTombstone.Deleted = true
		if err := qs.addToLog(ctx, tx, quadTombstone); err != nil {
			return err
		}
		node, err := qs.getPrimitiveFromLog(ctx, tx, prim.Subject)
		if err != nil {
			return err
		}
		node.Deleted = true
		return qs.addToLog(ctx, tx, node)
	})
	require.NoError(t, err)

	it := qs.QuadsAllIterator(ctx).Iterate(ctx)
	require.False(t, it.Next(ctx), "legacy deleted quad was returned by the all iterator")
	require.NoError(t, it.Err())
	require.NoError(t, it.Close())

	err = hkv.View(ctx, db, func(tx hkv.Tx) error {
		got, err := qs.hasPrimitive(ctx, tx, prim, true)
		require.NoError(t, err)
		require.Nil(t, got, "legacy deleted quad was returned by index lookup")

		val, err := qs.getValFromLog(ctx, tx, prim.Subject)
		require.NoError(t, err)
		require.Nil(t, val, "legacy deleted node was returned from the log")
		return nil
	})
	require.NoError(t, err)

	var callbacks int
	err = qs.IterateIndexPrefixNextRefs(ctx, DefaultQuadIndexes[0], []uint64{prim.Subject}, func(_ Int64Value, hasLive func() (bool, error)) error {
		callbacks++
		live, err := hasLive()
		require.NoError(t, err)
		require.False(t, live, "legacy deleted quad made an index prefix appear live")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, callbacks)
}

func TestMissingLogEntriesAreAbsent(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	q := quad.MakeIRI("missing-subject", "missing-predicate", "missing-object", "")
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Add})
	prim := onlyQuadPrimitive(t, ctx, db)

	err := hkv.Update(ctx, db, func(tx hkv.Tx) error {
		if err := tx.Del(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(prim.ID))); err != nil {
			return err
		}
		return tx.Del(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(prim.Subject)))
	})
	require.NoError(t, err)

	err = hkv.View(ctx, db, func(tx hkv.Tx) error {
		got, err := qs.hasPrimitive(ctx, tx, prim, true)
		require.NoError(t, err)
		require.Nil(t, got, "missing quad log entry was not treated as absent")

		val, err := qs.getValFromLog(ctx, tx, prim.Subject)
		require.NoError(t, err)
		require.Nil(t, val, "missing node log entry was not treated as absent")
		return nil
	})
	require.NoError(t, err)

	var callbacks int
	err = qs.IterateIndexPrefixNextRefs(ctx, DefaultQuadIndexes[0], []uint64{prim.Subject}, func(_ Int64Value, hasLive func() (bool, error)) error {
		callbacks++
		live, err := hasLive()
		require.NoError(t, err)
		require.False(t, live, "missing quad log entry made an index prefix appear live")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, callbacks)
}

func TestRemoveFromMapBucketNoOps(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	key := hkv.SKey("test-index", "posting")
	err := hkv.Update(ctx, db, func(tx hkv.Tx) error {
		counted := &countingTx{Tx: tx}
		if err := qs.removeFromMapBucket(ctx, counted, key, 2); err != nil {
			return err
		}
		require.Zero(t, counted.puts+counted.dels, "missing posting removal performed a write")
		original := appendIndex(nil, []uint64{1, 3})
		if err := tx.Put(ctx, key, original); err != nil {
			return err
		}
		if err := qs.removeFromMapBucket(ctx, counted, key, 2); err != nil {
			return err
		}
		require.Zero(t, counted.puts+counted.dels, "absent id removal performed a write")
		got, err := tx.Get(ctx, key)
		if err != nil {
			return err
		}
		require.Equal(t, original, []byte(got), "absent posting removal changed the posting list")
		return nil
	})
	require.NoError(t, err)
}

func TestRemoveFromMapBucketRejectsMalformedPosting(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	key := hkv.SKey("test-index", "malformed")
	err := hkv.Update(ctx, db, func(tx hkv.Tx) error {
		if err := tx.Put(ctx, key, []byte{0x80}); err != nil {
			return err
		}
		return qs.removeFromMapBucket(ctx, tx, key, 1)
	})
	require.Error(t, err, "malformed posting list was accepted")
}

func TestRemoveFromMapBucketPropagatesReadError(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	want := errors.New("posting read failed")
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		return qs.removeFromMapBucket(ctx, &getErrorTx{Tx: tx, err: want}, hkv.SKey("index", "key"), 1)
	})
	require.ErrorIs(t, err, want)
}

type countingTx struct {
	hkv.Tx
	puts int
	dels int
}

func (tx *countingTx) Put(ctx context.Context, key hkv.Key, value hkv.Value) error {
	tx.puts++
	return tx.Tx.Put(ctx, key, value)
}

func (tx *countingTx) Del(ctx context.Context, key hkv.Key) error {
	tx.dels++
	return tx.Tx.Del(ctx, key)
}

type getErrorTx struct {
	hkv.Tx
	err error
}

func (tx *getErrorTx) Get(context.Context, hkv.Key) (hkv.Value, error) {
	return nil, tx.err
}

func newReclaimTestStore(t testing.TB) (context.Context, *QuadStore, hkv.KV) {
	t.Helper()
	ctx := context.Background()
	db := btree.New()
	require.NoError(t, Init(ctx, db, nil))
	gqs, err := New(ctx, db, graph.Options{OptAssumeDefaultIdx: true})
	require.NoError(t, err)
	qs := gqs.(*QuadStore)
	t.Cleanup(func() { require.NoError(t, qs.Close()) })
	return ctx, qs, db
}

func applyReclaimTestDeltas(t testing.TB, ctx context.Context, qs *QuadStore, deltas ...graph.Delta) {
	t.Helper()
	require.NoError(t, qs.ApplyDeltas(ctx, deltas, graph.IgnoreOpts{}))
}

func onlyQuadPrimitive(t testing.TB, ctx context.Context, db hkv.KV) *proto.Primitive {
	t.Helper()
	prims := quadPrimitives(t, ctx, db)
	require.Len(t, prims, 1)
	return prims[0]
}

func newestQuadPrimitive(t testing.TB, ctx context.Context, db hkv.KV) *proto.Primitive {
	t.Helper()
	prims := quadPrimitives(t, ctx, db)
	require.NotEmpty(t, prims)
	newest := prims[0]
	for _, p := range prims[1:] {
		if p.ID > newest.ID {
			newest = p
		}
	}
	return newest
}

func quadPrimitives(t testing.TB, ctx context.Context, db hkv.KV) []*proto.Primitive {
	t.Helper()
	var out []*proto.Primitive
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		it := tx.Scan(ctx)
		defer it.Close()
		for it.Next(ctx) {
			key := it.Key()
			if len(key) != 2 || string(key[0]) != string(logIndex[0]) {
				continue
			}
			p := &proto.Primitive{}
			if err := p.UnmarshalVT(it.Val()); err != nil {
				return err
			}
			if !p.IsNode() {
				out = append(out, p)
			}
		}
		return it.Err()
	})
	require.NoError(t, err)
	return out
}

func countKVKeys(t testing.TB, ctx context.Context, db hkv.KV) int {
	t.Helper()
	var count int
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		it := tx.Scan(ctx)
		defer it.Close()
		for it.Next(ctx) {
			count++
		}
		return it.Err()
	})
	require.NoError(t, err)
	return count
}

// countLogAndPostingEntries returns the number of keys in the log bucket and
// the total number of ids listed across every quad index posting list.
func countLogAndPostingEntries(t testing.TB, ctx context.Context, db hkv.KV) (int, int) {
	t.Helper()
	buckets := make(map[string]struct{})
	for _, ind := range DefaultQuadIndexes {
		buckets[string(ind.bucket()[0])] = struct{}{}
	}
	var logKeys, postings int
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		it := tx.Scan(ctx)
		defer it.Close()
		for it.Next(ctx) {
			key := it.Key()
			if len(key) != 2 {
				continue
			}
			switch bucket := string(key[0]); {
			case bucket == string(logIndex[0]):
				logKeys++
			default:
				if _, ok := buckets[bucket]; !ok {
					continue
				}
				ids, err := decodeIndex(it.Val())
				if err != nil {
					return err
				}
				postings += len(ids)
			}
		}
		return it.Err()
	})
	require.NoError(t, err)
	return logKeys, postings
}

func readMetaInt(t testing.TB, ctx context.Context, db hkv.KV, name string) uint64 {
	t.Helper()
	var value int64
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		var err error
		value, err = (&QuadStore{}).getMetaIntTx(ctx, tx, name)
		return err
	})
	require.NoError(t, err)
	return uint64(value)
}
