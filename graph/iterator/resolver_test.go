package iterator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aperturerobotics/cayley/graph/graphmock"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

func TestResolverIteratorIterate(t *testing.T) {
	var ctx context.Context
	nodes := []quad.Value{
		quad.String("1"),
		quad.String("2"),
		quad.String("3"),
		quad.String("4"),
		quad.String("5"),
		quad.String("3"), // Assert iterator can handle duplicate values
	}
	data := make([]quad.Quad, 0, len(nodes))
	for _, node := range nodes {
		data = append(data, quad.Make(quad.String("0"), "has", node, nil))
	}
	qs := &graphmock.Store{
		Data: data,
	}
	expected := make(map[quad.Value]refs.Ref)
	for _, node := range nodes {
		var err error
		expected[node], err = qs.ValueOf(ctx, node)
		require.NoError(t, err)
	}
	it := iterator.NewResolver(qs, nodes...).Iterate(ctx)
	for _, node := range nodes {
		require.True(t, it.Next(ctx))
		require.NoError(t, it.Err())
		resi, err := it.Result(ctx)
		require.NoError(t, err)
		require.Equal(t, expected[node], resi)
	}
	require.False(t, it.Next(ctx))
	resi, err := it.Result(ctx)
	require.NoError(t, err)
	require.Nil(t, resi)
}

func TestResolverIteratorNotFoundError(t *testing.T) {
	var ctx context.Context
	nodes := []quad.Value{
		quad.String("1"),
		quad.String("2"),
		quad.String("3"),
		quad.String("4"),
		quad.String("5"),
	}
	data := make([]quad.Quad, 0)
	skip := 3
	for i, node := range nodes {
		// Simulate a missing subject
		if i == skip {
			continue
		}
		data = append(data, quad.Make(quad.String("0"), "has", node, nil))
	}
	qs := &graphmock.Store{
		Data: data,
	}
	count := 0
	it := iterator.NewResolver(qs, nodes...).Iterate(ctx)
	for it.Next(ctx) {
		count++
	}
	require.Equal(t, 0, count)
	require.Error(t, it.Err())
	resi, err := it.Result(ctx)
	require.Error(t, err)
	require.Nil(t, resi)
}

func TestResolverIteratorContains(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []quad.Value
		subject  quad.Value
		contains bool
	}{
		{
			"contains",
			[]quad.Value{
				quad.String("1"),
				quad.String("2"),
				quad.String("3"),
			},
			quad.String("2"),
			true,
		},
		{
			"not contains",
			[]quad.Value{
				quad.String("1"),
				quad.String("3"),
			},
			quad.String("2"),
			false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ctx context.Context
			data := make([]quad.Quad, 0, len(test.nodes))
			for _, node := range test.nodes {
				data = append(data, quad.Make(quad.String("0"), "has", node, nil))
			}
			qs := &graphmock.Store{
				Data: data,
			}
			it := iterator.NewResolver(qs, test.nodes...).Lookup(ctx)
			cnt, err := it.Contains(ctx, refs.PreFetched(test.subject))
			require.NoError(t, err)
			require.Equal(t, test.contains, cnt)
		})
	}
}
