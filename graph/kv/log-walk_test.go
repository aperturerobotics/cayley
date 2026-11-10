package kv

import (
	"fmt"
	"slices"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/pquads"
	"github.com/stretchr/testify/require"
)

// TestWalkLogYieldsLiveGraph checks that the walk's nodes and quads rebuild
// exactly the live quads, after deletes that reclaim log entries, and with
// more than ten IDs so decimal key order differs from numeric order.
func TestWalkLogYieldsLiveGraph(t *testing.T) {
	ctx, qs, db := newReclaimTestStore(t)
	var live []quad.Quad
	for i := range 12 {
		q := quad.MakeIRI(fmt.Sprintf("s%d", i%3), "p", fmt.Sprintf("o%d", i), "")
		if i%4 == 0 {
			q.Label = quad.IRI("l")
		}
		applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Add})
		live = append(live, q)
	}
	for _, q := range []quad.Quad{live[1], live[8]} {
		applyReclaimTestDeltas(t, ctx, qs, graph.Delta{Quad: q, Action: graph.Delete})
	}
	live = slices.Delete(slices.Delete(live, 8, 9), 1, 2)

	// Collect the walk, then resolve each quad's IDs through its nodes.
	values := make(map[uint64]quad.Value)
	var links []*proto.Primitive
	err := WalkLog(ctx, db, func(p *proto.Primitive) error {
		if !p.IsNode() {
			links = append(links, p.CloneVT())
			return nil
		}
		v, err := pquads.UnmarshalValue(ctx, p.Value)
		values[p.ID] = v
		return err
	})
	require.NoError(t, err)
	var got []string
	for _, p := range links {
		q := quad.Quad{}
		for _, d := range quad.Directions {
			if id := p.GetDirection(d); id != 0 {
				require.Contains(t, values, id, "quad %d names a node the walk did not yield", p.ID)
				q.Set(d, values[id])
			}
		}
		got = append(got, q.String())
	}
	want := make([]string, len(live))
	for i, q := range live {
		want[i] = q.String()
	}
	require.ElementsMatch(t, want, got)

	// ReadLog returns the walked entries by ID and nil for a removed one, and
	// LogKeyID parses the keys under LogPrefix.
	ids := []uint64{links[0].ID, 1 << 40}
	err = kv.View(ctx, db, func(tx kv.Tx) error {
		got, err := ReadLog(ctx, tx, ids)
		if err != nil {
			return err
		}
		require.True(t, got[0].EqualVT(links[0]))
		require.Nil(t, got[1])
		return nil
	})
	require.NoError(t, err)
	id, ok := LogKeyID(LogPrefix().AppendBytes([]byte("12")))
	require.True(t, ok)
	require.Equal(t, uint64(12), id)
	_, ok = LogKeyID(kv.Key{[]byte("m"), []byte("12")})
	require.False(t, ok)
}
