package kv

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"
	"strconv"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/pkg/errors"
)

// TestIndexRebuilder preserves live quads and counters while replacing postings.
func TestIndexRebuilder(t *testing.T) {
	// Open a graph with the default forward posting layout.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	g, err := New(ctx, db, graph.Options{OptAssumeDefaultIdx: true})
	if err != nil {
		t.Fatal(err)
	}
	qs := g.(*QuadStore)

	// Populate multiple log batches under one shared forward posting.
	deltas := make([]graph.Delta, indexRebuildBatchSize+1)
	for i := range deltas {
		deltas[i] = graph.Delta{
			Quad:   quad.MakeIRI("subject", "predicate", "object/"+strconv.Itoa(i), ""),
			Action: graph.Add,
		}
	}
	if err := qs.ApplyDeltas(ctx, deltas, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
	if err := qs.ApplyDeltas(ctx, []graph.Delta{{Quad: deltas[0].Quad, Action: graph.Delete}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}

	// Include a deleted primitive inside the horizon to exercise its exclusion.
	deletedID := uint64(qs.horizon(ctx)) + 1
	err = kv.Update(ctx, db, func(tx kv.Tx) error {
		data, err := (&proto.Primitive{ID: deletedID, Subject: 1, Predicate: 2, Object: 3, Deleted: true}).MarshalVT()
		if err != nil {
			return err
		}
		if err := tx.Put(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(deletedID)), data); err != nil {
			return err
		}
		return tx.Put(ctx, metaBucket.AppendBytes([]byte("horizon")), binary.LittleEndian.AppendUint64(nil, deletedID))
	})
	if err != nil {
		t.Fatal(err)
	}
	before := indexRebuildProtectedValues(t, db)
	indexes := []QuadIndex{
		{Dirs: []quad.Direction{quad.Subject, quad.Predicate, quad.Object}},
		{Dirs: []quad.Direction{quad.Object, quad.Predicate, quad.Subject}},
		{Dirs: []quad.Direction{quad.Predicate}},
	}

	// Rebuild twice to require a complete replacement, including after reopening.
	for range 2 {
		// Simulate a partial desired index left by an interrupted rebuild.
		err := kv.Update(ctx, db, func(tx kv.Tx) error {
			return tx.Put(ctx, kv.Key{[]byte("spo"), []byte("incomplete")}, appendIndex(nil, []uint64{deletedID}))
		})
		if err != nil {
			t.Fatal(err)
		}

		// Replace all postings, even when the stored metadata still names the old layout.
		r, err := NewIndexRebuilder(db, indexes)
		if err != nil {
			t.Fatal(err)
		}
		n, err := r.Rebuild(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n != uint64(len(deltas)-1) {
			t.Fatalf("rebuilt %d quads, want %d", n, len(deltas)-1)
		}
		g, err = New(ctx, db, nil)
		if err != nil {
			t.Fatal(err)
		}
		qs = g.(*QuadStore)
		if !CompareQuadIndexes(qs.indexes.all, indexes) {
			t.Fatalf("reopened indexes = %v", qs.indexes.all)
		}
	}

	// Require per-edge forward postings and unchanged authoritative values.
	after := indexRebuildProtectedValues(t, db)
	if !maps.EqualFunc(before, after, bytes.Equal) {
		t.Fatal("rebuild changed primitive log, nodes, or counters")
	}
	var forward, shared int
	err = kv.View(ctx, db, func(tx kv.Tx) error {
		// Check that bounded forward postings replaced the obsolete layout.
		it := tx.Scan(ctx)
		var postingErr error
		for it.Next(ctx) {
			key := it.Key()
			if string(key[0]) == "sp" {
				postingErr = errors.New("obsolete forward posting remains")
				break
			}
			if string(key[0]) == "p" {
				shared++
				ids, err := decodeIndex(it.Val())
				if err != nil {
					postingErr = err
					break
				}
				if len(ids) != len(deltas)-1 || !slices.IsSorted(ids) || slices.Contains(ids, deletedID) {
					postingErr = errors.New("shared posting lost numeric order or included deleted primitives")
					break
				}
			}
			if string(key[0]) == "spo" {
				_, n := binary.Uvarint(it.Val())
				if n <= 0 || n != len(it.Val()) {
					postingErr = errors.Errorf("forward posting has more than one quad: %v", key)
					break
				}
				forward++
			}
		}
		scanErr := it.Err()
		closeErr := it.Close()
		if postingErr != nil {
			return postingErr
		}
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if forward != len(deltas)-1 || shared != 1 {
		t.Fatalf("forward/shared postings = %d/%d", forward, shared)
	}

	// Resolve the subject through the reopened graph.
	ref, err := qs.ValueOf(ctx, quad.IRI("subject"))
	if err != nil {
		t.Fatal(err)
	}

	// Count outgoing quads through the configured forward index.
	it := qs.QuadIterator(ctx, quad.Subject, ref).Iterate(ctx)
	var count int
	for it.Next(ctx) {
		count++
	}
	if err := it.Err(); err != nil {
		t.Fatal(err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if count != len(deltas)-1 {
		t.Fatalf("outgoing quads = %d", count)
	}

	// Delete a surviving quad through the rebuilt graph's write path.
	if err := qs.ApplyDeltas(ctx, []graph.Delta{{Quad: deltas[1].Quad, Action: graph.Delete}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
}

// TestIndexRebuilderRejectsEmptyDirections prevents empty prefixes from erasing graph data.
func TestIndexRebuilderRejectsEmptyDirections(t *testing.T) {
	// Reject an empty desired index before it can become a scan prefix.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := NewIndexRebuilder(db, []QuadIndex{{}}); err == nil {
		t.Fatal("accepted an index with no directions")
	}

	// Store one quad and invalid old metadata without changing its postings.
	g, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	qs := g.(*QuadStore)
	if err := qs.ApplyDeltas(ctx, []graph.Delta{{
		Quad: quad.MakeIRI("subject", "predicate", "object", ""), Action: graph.Add,
	}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
	qs.setLayout([]QuadIndex{{}})
	if err := qs.writeIndexesMeta(ctx); err != nil {
		t.Fatal(err)
	}
	before := indexRebuildProtectedValues(t, db)

	// Refuse the unsafe old layout before deleting any authoritative values.
	r, err := NewIndexRebuilder(db, DefaultQuadIndexes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Rebuild(ctx); err == nil {
		t.Fatal("rebuilt an index with an empty stored prefix")
	}
	if !maps.EqualFunc(before, indexRebuildProtectedValues(t, db), bytes.Equal) {
		t.Fatal("invalid stored indexes changed authoritative values")
	}
}

// TestIndexRebuilderMissingHorizon preserves postings when a nonempty log has no horizon.
func TestIndexRebuilderMissingHorizon(t *testing.T) {
	// Create a graph with a live primitive and its posting.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	g, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.ApplyDeltas(ctx, []graph.Delta{{
		Quad: quad.MakeIRI("subject", "predicate", "object", ""), Action: graph.Add,
	}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}

	// Remove the horizon and require validation before any destructive work.
	if err := kv.Update(ctx, db, func(tx kv.Tx) error {
		return tx.Del(ctx, metaBucket.AppendBytes([]byte("horizon")))
	}); err != nil {
		t.Fatal(err)
	}
	before := indexRebuildProtectedValues(t, db)
	r, err := NewIndexRebuilder(db, DefaultQuadIndexes)
	if err != nil {
		t.Fatal(err)
	}
	n, err := r.Rebuild(ctx)
	if err == nil || err.Error() != "missing primitive horizon for nonempty log" || n != 0 {
		t.Fatalf("rebuild = %d, %v", n, err)
	}
	if !maps.EqualFunc(before, indexRebuildProtectedValues(t, db), bytes.Equal) {
		t.Fatal("missing horizon changed authoritative values")
	}

	// Require the original forward posting to remain readable.
	ref, err := g.ValueOf(ctx, quad.IRI("subject"))
	if err != nil {
		t.Fatal(err)
	}
	it := g.QuadIterator(ctx, quad.Subject, ref).Iterate(ctx)
	if !it.Next(ctx) {
		t.Fatal("missing horizon removed the live posting")
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestIndexRebuilderInterrupted blocks every open mode until a same-layout rebuild succeeds.
func TestIndexRebuilderInterrupted(t *testing.T) {
	// Populate a graph whose layout already matches the rebuild target.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	g, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.ApplyDeltas(ctx, []graph.Delta{{
		Quad: quad.MakeIRI("subject", "predicate", "object", ""), Action: graph.Add,
	}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}

	// Force the rebuild to fail after its posting-clearing commits.
	key := logIndex.AppendBytes(uint64KeyBytesBase10(1))
	var saved []byte
	if err := kv.Update(ctx, db, func(tx kv.Tx) error {
		val, err := tx.Get(ctx, key)
		if err != nil {
			return err
		}
		saved = bytes.Clone(val)
		return tx.Put(ctx, key, []byte{0xff})
	}); err != nil {
		t.Fatal(err)
	}
	r, err := NewIndexRebuilder(db, DefaultQuadIndexes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Rebuild(ctx); err == nil {
		t.Fatal("rebuilt a corrupt primitive")
	}

	// Reject metadata, default, and explicit opens while postings are incomplete.
	for _, opts := range []graph.Options{nil, {OptAssumeDefaultIdx: true}, {OptQuadIndexes: DefaultQuadIndexes}} {
		if _, err := New(ctx, db, opts); err == nil {
			t.Fatalf("opened an incomplete rebuild with options %v", opts)
		}
	}

	// Restore the primitive and require rerunning to publish a usable layout.
	if err := kv.Update(ctx, db, func(tx kv.Tx) error {
		return tx.Put(ctx, key, saved)
	}); err != nil {
		t.Fatal(err)
	}
	if n, err := r.Rebuild(ctx); err != nil || n != 1 {
		t.Fatalf("rerun = %d, %v", n, err)
	}
	if _, err := New(ctx, db, graph.Options{OptQuadIndexes: DefaultQuadIndexes}); err != nil {
		t.Fatal(err)
	}
}

// indexRebuildProtectedValues snapshots the values an index rebuild must preserve.
func indexRebuildProtectedValues(t *testing.T, db kv.KV) map[string][]byte {
	// Snapshot authoritative values without retaining the transaction's buffers.
	t.Helper()
	values := make(map[string][]byte)
	err := kv.View(t.Context(), db, func(tx kv.Tx) error {
		// Collect only the primitive, node, reference-count, and counter buckets.
		it := tx.Scan(t.Context())
		for it.Next(t.Context()) {
			key := it.Key()
			if slices.Contains([]string{"l", "v", "n", "m"}, string(key[0])) {
				values[string(key[0])+":"+string(key[1])] = slices.Clone(it.Val())
			}
		}
		err := it.Err()
		closeErr := it.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return values
}

// TestIndexRebuilderResume reruns an interrupted rebuild from its last
// committed batch and merges into postings written before the interruption.
func TestIndexRebuilderResume(t *testing.T) {
	// Populate more than one batch under one shared predicate posting.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	g, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	qs := g.(*QuadStore)

	// Add enough quads, with their object nodes, to fill several batches.
	deltas := make([]graph.Delta, indexRebuildBatchSize)
	for i := range deltas {
		deltas[i] = graph.Delta{
			Quad:   quad.MakeIRI("subject", "predicate", "object/"+strconv.Itoa(i), ""),
			Action: graph.Add,
		}
	}
	if err := qs.ApplyDeltas(ctx, deltas, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
	horizon := uint64(qs.horizon(ctx))

	// Fail the batch holding the last primitive after earlier batches commit.
	key := logIndex.AppendBytes(uint64KeyBytesBase10(horizon))
	var saved []byte
	if err := kv.Update(ctx, db, func(tx kv.Tx) error {
		val, err := tx.Get(ctx, key)
		if err != nil {
			return err
		}
		saved = bytes.Clone(val)
		return tx.Put(ctx, key, []byte{0xff})
	}); err != nil {
		t.Fatal(err)
	}
	indexes := []QuadIndex{
		{Dirs: []quad.Direction{quad.Subject, quad.Predicate, quad.Object}},
		{Dirs: []quad.Direction{quad.Predicate}},
	}
	r, err := NewIndexRebuilder(db, indexes)
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Rebuild(ctx)
	if err == nil || first == 0 {
		t.Fatalf("interrupted rebuild = %d, %v", first, err)
	}

	// Resume with a new rebuilder and require only the remaining quads.
	if err := kv.Update(ctx, db, func(tx kv.Tx) error {
		return tx.Put(ctx, key, saved)
	}); err != nil {
		t.Fatal(err)
	}
	r, err = NewIndexRebuilder(db, indexes)
	if err != nil {
		t.Fatal(err)
	}
	rest, err := r.Rebuild(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first+rest != uint64(len(deltas)) {
		t.Fatalf("indexed %d then %d quads, want %d in total", first, rest, len(deltas))
	}

	// Require the shared posting to hold every quad once, in order.
	var ids []uint64
	err = kv.View(ctx, db, func(tx kv.Tx) error {
		it := tx.Scan(ctx)
		for it.Next(ctx) {
			if string(it.Key()[0]) != "p" {
				continue
			}
			if ids != nil {
				return errors.New("more than one predicate posting")
			}
			var err error
			if ids, err = decodeIndex(it.Val()); err != nil {
				return err
			}
		}
		return it.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(deltas) || !slices.IsSorted(ids) || len(slices.Compact(slices.Clone(ids))) != len(ids) {
		t.Fatalf("predicate posting has %d ids, want %d unique in order", len(ids), len(deltas))
	}
}
