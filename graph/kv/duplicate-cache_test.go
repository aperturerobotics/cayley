package kv

import (
	"context"
	"strconv"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	hkv "github.com/aperturerobotics/cayley/kv"
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
		_, err := w.WriteQuads(ctx, []quad.Quad{q})
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.Len(t, quadPrimitives(t, ctx, db), 1)
	applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Delete})
	logs, postings := countLogAndPostingEntries(t, ctx, db)
	require.Zero(t, logs)
	require.Zero(t, postings)
}

// TestUnchangedDeltasDoNotCommit checks that a batch of ignored duplicates and
// missing removals commits nothing, on both sides of the existence cache.
func TestUnchangedDeltasDoNotCommit(t *testing.T) {
	ctx := context.Background()
	db := &commitCountingKV{KV: btree.New()}
	require.NoError(t, Init(ctx, db, nil))
	gqs, err := New(ctx, db, graph.Options{OptAssumeDefaultIdx: true})
	require.NoError(t, err)
	qs := gqs.(*QuadStore)
	t.Cleanup(func() { require.NoError(t, qs.Close()) })

	// Store more quads than the existence cache holds, then replay them with
	// a removal of a quad between stored nodes that was never added.
	deltas := make([]graph.Delta, 2001, 2002)
	for i := range deltas {
		deltas[i] = graph.Delta{Quad: quad.MakeIRI("owner", "ref", "block-"+strconv.Itoa(i), ""), Action: graph.Add}
	}
	opts := graph.IgnoreOpts{IgnoreDup: true, IgnoreMissing: true}
	require.NoError(t, qs.ApplyDeltas(ctx, deltas, opts))
	missing := graph.Delta{Quad: quad.MakeIRI("block-0", "ref", "block-1", ""), Action: graph.Delete}
	deltas = append(deltas, missing)
	before := db.commits
	for _, batch := range [][]graph.Delta{deltas, deltas[len(deltas)-2:]} {
		require.NoError(t, qs.ApplyDeltas(ctx, batch, opts))
	}
	require.Equal(t, before, db.commits)

	// A real change still commits.
	remove := graph.Delta{Quad: deltas[0].Quad, Action: graph.Delete}
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{remove}, opts))
	require.Equal(t, before+1, db.commits)
}

// commitCountingKV counts committed transactions.
type commitCountingKV struct {
	hkv.KV
	commits int
}

// Tx opens a transaction whose commits are counted.
func (c *commitCountingKV) Tx(ctx context.Context, rw bool) (hkv.Tx, error) {
	tx, err := c.KV.Tx(ctx, rw)
	if err != nil {
		return nil, err
	}
	return &commitCountingTx{Tx: tx, kv: c}, nil
}

// commitCountingTx counts its commit on the owning store.
type commitCountingTx struct {
	hkv.Tx
	kv *commitCountingKV
}

// Commit commits the transaction and counts it.
func (t *commitCountingTx) Commit(ctx context.Context) error {
	t.kv.commits++
	return t.Tx.Commit(ctx)
}
