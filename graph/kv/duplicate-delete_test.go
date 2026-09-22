package kv

import (
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	graphlog "github.com/aperturerobotics/cayley/graph/log"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/graph/refs"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// TestDeleteHistoricalDuplicateQuads verifies that one logical removal reclaims
// every stored copy while preserving shared nodes and distinctly labeled quads.
func TestDeleteHistoricalDuplicateQuads(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	q := quad.MakeIRI("shared", "ref", "shared", "")
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Add})
	original := onlyQuadPrimitive(t, ctx, db)

	// Reproduce an older store with valid indexes and counts for 69 copies.
	require.NoError(t, hkv.Update(ctx, db, func(tx hkv.Tx) error {
		cache := newMetaCache()
		const copies = 68
		start, err := qs.genIDs(ctx, tx, cache, copies)
		if err != nil {
			return err
		}
		links := make([]*proto.Primitive, copies)
		adds := make([]graph.Delta, copies)
		for i := range links {
			links[i] = original.CloneVT()
			links[i].ID = start + uint64(i)
			adds[i] = graph.Delta{Quad: q, Action: graph.Add}
		}
		if err := qs.indexLinks(ctx, tx, cache, links, nil); err != nil {
			return err
		}
		nodes := make(map[refs.ValueHash]resolvedNode)
		for _, dir := range quad.Directions {
			if value := q.Get(dir); value != nil {
				nodes[refs.HashOf(value)] = resolvedNode{ID: original.GetDirection(dir)}
			}
		}
		if err := qs.applyNodeDeltas(ctx, tx, graphlog.SplitDeltas(adds), nodes); err != nil {
			return err
		}
		if err := qs.flushMapBucket(ctx, tx); err != nil {
			return err
		}
		return qs.flushMetaCache(ctx, tx, cache)
	}))
	keep := quad.MakeIRI("shared", "ref", "shared", "keep")
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: keep, Action: graph.Add})
	require.Len(t, quadPrimitives(t, ctx, db), 70)

	// A repeated request is a no-op after the first logical deletion.
	remove := graph.Delta{Quad: q, Action: graph.Delete}
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{remove, remove}, graph.IgnoreOpts{IgnoreMissing: true}))
	remaining := onlyQuadPrimitive(t, ctx, db)
	actual, err := qs.Quad(ctx, remaining)
	require.NoError(t, err)
	require.Equal(t, keep, actual)
	require.EqualValues(t, 1, readMetaInt(t, ctx, db, "size"))
	require.Empty(t, countDanglingPostings(t, ctx, db))

	// Removing the surviving quad must also reclaim all shared node records.
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: keep, Action: graph.Delete})
	logs, postings := countLogAndPostingEntries(t, ctx, db)
	require.Zero(t, logs)
	require.Zero(t, postings)
}
