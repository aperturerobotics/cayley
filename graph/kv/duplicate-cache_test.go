package kv

import (
	"strconv"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// TestDuplicateAddsAcrossCacheEviction checks a batch mixing remembered and
// evicted quads. The existence cache must never bypass durable deduplication.
func TestDuplicateAddsAcrossCacheEviction(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	adds := make([]graph.Delta, 2001)
	for i := range adds {
		adds[i] = graph.Delta{Quad: quad.MakeIRI("owner", "ref", "block-"+strconv.Itoa(i), ""), Action: graph.Add}
	}
	applyReclaimTestDeltas(t, ctx, qs, adds...)
	before := countKVKeys(t, ctx, db)
	horizon := readMetaInt(t, ctx, db, "horizon")
	for range 3 {
		require.NoError(t, qs.ApplyDeltas(ctx, adds, graph.IgnoreOpts{IgnoreDup: true}))
		require.EqualValues(t, len(adds), readMetaInt(t, ctx, db, "size"))
		require.Equal(t, before, countKVKeys(t, ctx, db))
		require.Equal(t, horizon, readMetaInt(t, ctx, db, "horizon"))
	}
}

// TestQuadWriterDeduplicatesPendingQuads covers separate input batches within
// the writer's transaction, before its buffered postings reach the KV store.
func TestQuadWriterDeduplicatesPendingQuads(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	q := quad.MakeIRI("owner", "ref", "block", "")
	w, err := qs.NewQuadWriter(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, w.Close()) })
	for range 3 {
		require.NoError(t, w.WriteQuad(ctx, q))
	}
	require.NoError(t, w.Close())
	require.Len(t, quadPrimitives(t, ctx, db), 1)
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Delete})
	logs, postings := countLogAndPostingEntries(t, ctx, db)
	require.Zero(t, logs)
	require.Zero(t, postings)
}
