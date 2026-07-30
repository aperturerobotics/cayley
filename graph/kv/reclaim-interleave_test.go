package kv

import (
	"context"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// countDanglingPostings returns posting ids with no log entry.
func countDanglingPostings(t testing.TB, ctx context.Context, db hkv.KV) []uint64 {
	t.Helper()
	buckets := make(map[string]struct{})
	for _, ind := range DefaultQuadIndexes {
		buckets[string(ind.bucket()[0])] = struct{}{}
	}
	logIDs := make(map[string]struct{})
	var postingIDs []uint64
	var postingKeys [][]byte
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		it := tx.Scan(ctx)
		defer it.Close()
		for it.Next(ctx) {
			key := it.Key()
			if len(key) != 2 {
				continue
			}
			bucket := string(key[0])
			if bucket == string(logIndex[0]) {
				logIDs[string(key[1])] = struct{}{}
				continue
			}
			if _, ok := buckets[bucket]; !ok {
				continue
			}
			ids, err := decodeIndex(it.Val())
			if err != nil {
				return err
			}
			for _, id := range ids {
				postingIDs = append(postingIDs, id)
				postingKeys = append(postingKeys, uint64KeyBytesBase10(id))
			}
		}
		return it.Err()
	})
	require.NoError(t, err)
	var dangling []uint64
	for i, k := range postingKeys {
		if _, ok := logIDs[string(k)]; !ok {
			dangling = append(dangling, postingIDs[i])
		}
	}
	return dangling
}

// TestAddAndDeleteSameQuadInOneApplyIsRefused pins the reason reclamation can
// ignore the buffered posting map. An add buffers its posting in memory until
// the flush at the end of the transaction, while a removal reads the
// transaction directly, so a delete that lands in the same batch as its own add
// would strip nothing and leave a posting pointing at a deleted log entry. That
// pair never reaches the index because the delete is refused first.
func TestAddAndDeleteSameQuadInOneApplyIsRefused(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	q := quad.MakeIRI("s", "p", "o", "")
	err := qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: q, Action: graph.Add},
		{Quad: q, Action: graph.Delete},
	}, graph.IgnoreOpts{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quad does not exist")
	if d := countDanglingPostings(t, ctx, db); len(d) != 0 {
		t.Fatalf("dangling posting ids: %v", d)
	}
}

// TestDeleteAndAddSharingIndexKeyInOneApply covers a delete and an add whose
// quads share an index posting key. The delete rewrites that posting through
// the transaction and the add appends to it during the flush, so the surviving
// quad has to remain listed and the removed one has to be gone.
func TestDeleteAndAddSharingIndexKeyInOneApply(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	first := quad.MakeIRI("s", "p", "o1", "")
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: first, Action: graph.Add})

	second := quad.MakeIRI("s", "p", "o2", "")
	err := qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: second, Action: graph.Add},
		{Quad: first, Action: graph.Delete},
	}, graph.IgnoreOpts{})
	require.NoError(t, err)

	logKeys, postings := countLogAndPostingEntries(t, ctx, db)
	// One quad, its subject, predicate, and remaining object.
	require.Equal(t, 4, logKeys)
	// The surviving quad is listed once per index.
	require.Equal(t, len(qs.indexes.all), postings)
	if d := countDanglingPostings(t, ctx, db); len(d) != 0 {
		t.Fatalf("dangling posting ids: %v", d)
	}
	// The surviving quad must still be findable.
	ref, err := qs.ValueOf(ctx, quad.IRI("o2"))
	require.NoError(t, err)
	require.NotNil(t, ref)
}
