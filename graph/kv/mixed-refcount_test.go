package kv

import (
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

// TestMixedReferenceChanges retains complete quads and exact node counts when
// additions share nodes with more removals, ignored duplicates, or missing edges.
func TestMixedReferenceChanges(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			name := "new"
			if duplicate {
				name = "duplicate"
			}
			if missing {
				name += "-missing"
			}
			t.Run(name, func(t *testing.T) {
				ctx, qs, db := newReclaimTestStore(t)
				a := quad.MakeIRI("a", "ref", "shared", "")
				b := quad.MakeIRI("b", "ref", "shared", "")
				keep := quad.MakeIRI("keep", "ref", "shared", "")
				applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: a, Action: graph.Add}, graph.Delta{Quad: b, Action: graph.Add})
				if duplicate {
					applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: keep, Action: graph.Add})
				}
				changes := []graph.Delta{{Quad: keep, Action: graph.Add}, {Quad: a, Action: graph.Delete}, {Quad: b, Action: graph.Delete}}
				if missing {
					changes = append(changes, graph.Delta{Quad: quad.MakeIRI("absent", "ref", "shared", ""), Action: graph.Delete})
				}
				require.NoError(t, qs.ApplyDeltas(ctx, changes, graph.IgnoreOpts{IgnoreDup: true, IgnoreMissing: true}))
				prim := onlyQuadPrimitive(t, ctx, db)
				actual, err := qs.Quad(ctx, prim)
				require.NoError(t, err)
				require.Equal(t, keep, actual)
				require.Empty(t, countDanglingPostings(t, ctx, db))
				applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: keep, Action: graph.Delete})
				logs, postings := countLogAndPostingEntries(t, ctx, db)
				require.Zero(t, logs, "node reference counts must permit complete reclamation")
				require.Zero(t, postings)
			})
		}
	}
}
