package kv

import (
	"fmt"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// A shared ownership index is rewritten once for a removal batch. Surviving
// links and their log records remain queryable; removed links are reclaimed.
func TestReclaimSharedPostingBatch(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	adds := sharedPostingDeltas(1024)
	applyReclaimTestDeltas(t, ctx, qs, adds...)
	links := quadPrimitives(t, ctx, db)[:256]
	var writes int
	err := hkv.Update(ctx, db, func(tx hkv.Tx) error {
		counted := &countingTx{Tx: tx}
		if err := qs.markLinksDead(ctx, counted, newMetaCache(), links); err != nil {
			return err
		}
		writes = counted.puts
		return nil
	})
	require.NoError(t, err)
	// Each reverse posting and quad log is deleted. Only the shared forward
	// posting requires a rewrite, regardless of the number of removed links.
	require.Equal(t, 1, writes)
	_, postings := countLogAndPostingEntries(t, ctx, db)
	require.Equal(t, (len(adds)-len(links))*len(qs.indexes.all), postings)
	require.Empty(t, countDanglingPostings(t, ctx, db))
}

func BenchmarkReclaimSharedPostingBatch(b *testing.B) {
	for _, size := range []int{4096, 65536} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			ctx, qs, _ := newReclaimTestStore(b)
			adds := sharedPostingDeltas(size)
			applyReclaimTestDeltas(b, ctx, qs, adds...)
			removes := make([]graph.Delta, 512)
			for i := range removes {
				removes[i] = graph.Delta{Quad: adds[i].Quad, Action: graph.Delete}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				applyReclaimTestDeltas(b, ctx, qs, removes...)
				b.StopTimer()
				applyReclaimTestDeltas(b, ctx, qs, adds[:len(removes)]...)
				b.StartTimer()
			}
		})
	}
}

func sharedPostingDeltas(count int) []graph.Delta {
	deltas := make([]graph.Delta, count)
	for i := range deltas {
		deltas[i] = graph.Delta{
			Quad:   quad.MakeIRI("staging-owner", "gc/ref", fmt.Sprint("block-", i), ""),
			Action: graph.Add,
		}
	}
	return deltas
}
