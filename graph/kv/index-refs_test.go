package kv

import (
	"context"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

func TestIterateIndexPrefixNextRefs(t *testing.T) {
	ctx := context.Background()
	gqs, err := New(ctx, btree.New(), graph.Options{OptAssumeDefaultIdx: true})
	require.NoError(t, err)

	qs, ok := gqs.(*QuadStore)
	require.True(t, ok)

	qw, err := graph.NewQuadWriter("single", qs, nil)
	require.NoError(t, err)
	defer qw.Close()

	require.NoError(t, qw.AddQuad(ctx, quad.Make(quad.IRI("a"), quad.IRI("p"), quad.IRI("x"), nil)))
	require.NoError(t, qw.AddQuad(ctx, quad.Make(quad.IRI("b"), quad.IRI("p"), quad.IRI("x"), nil)))
	require.NoError(t, qw.AddQuad(ctx, quad.Make(quad.IRI("c"), quad.IRI("p"), quad.IRI("y"), nil)))

	predRef, err := qs.ValueOf(ctx, quad.IRI("p"))
	require.NoError(t, err)
	objRef, err := qs.ValueOf(ctx, quad.IRI("x"))
	require.NoError(t, err)

	predID, ok := predRef.(Int64Value)
	require.True(t, ok)
	objID, ok := objRef.(Int64Value)
	require.True(t, ok)

	var got []refs.Ref
	err = qs.IterateIndexPrefixNextRefs(
		ctx,
		DefaultQuadIndexes[1],
		[]uint64{uint64(objID), uint64(predID)},
		func(ref Int64Value, hasLive func() (bool, error)) error {
			live, err := hasLive()
			require.NoError(t, err)
			require.True(t, live)
			got = append(got, ref)
			return nil
		},
	)
	require.NoError(t, err)
	require.Len(t, got, 2)

	vals, err := graph.ValuesOf(ctx, qs, got)
	require.NoError(t, err)
	require.ElementsMatch(t, []quad.Value{quad.IRI("a"), quad.IRI("b")}, vals)
}
