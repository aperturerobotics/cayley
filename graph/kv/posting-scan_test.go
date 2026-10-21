package kv_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/writer"
)

// newPostingScanStore returns a store holding links by predicate p and q, some
// labeled and some removed, so a lookup by predicate has no index prefix.
func newPostingScanStore(ctx context.Context, t *testing.T) *kv.QuadStore {
	// Open an empty store.
	t.Helper()
	kdb := btree.New()
	require.NoError(t, kv.Init(ctx, kdb, nil))
	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	t.Cleanup(func() { gqs.Close() })
	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	// Write the links, then remove one.
	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)
	for _, q := range []quad.Quad{
		quad.MakeIRI("a", "p", "b", ""),
		quad.MakeIRI("a", "q", "b", ""),
		quad.MakeIRI("c", "p", "d", "l"),
		quad.MakeIRI("e", "p", "f", ""),
		quad.MakeIRI("g", "p", "h", "l"),
		quad.MakeIRI("i", "p", "j", ""),
	} {
		require.NoError(t, qw.AddQuad(ctx, q))
	}
	require.NoError(t, qw.RemoveQuad(ctx, quad.MakeIRI("e", "p", "f", "")))
	return qs
}

func TestCollectFilteredQuadsBatchByPredicate(t *testing.T) {
	// Build the store.
	ctx := context.Background()
	qs := newPostingScanStore(ctx, t)

	// By predicate alone the live links come back in the order they were added.
	results, err := qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("", "p", "", ""),
	}, 0)
	require.NoError(t, err)
	require.Equal(t, [][]quad.Quad{{
		quad.MakeIRI("a", "p", "b", ""),
		quad.MakeIRI("c", "p", "d", "l"),
		quad.MakeIRI("g", "p", "h", "l"),
		quad.MakeIRI("i", "p", "j", ""),
	}}, results)

	// A limit keeps the earliest links, as the log scan did.
	results, err = qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("", "p", "", ""),
	}, 2)
	require.NoError(t, err)
	require.Equal(t, [][]quad.Quad{{
		quad.MakeIRI("a", "p", "b", ""),
		quad.MakeIRI("c", "p", "d", "l"),
	}}, results)

	// By predicate and label only the labeled links match.
	results, err = qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("", "p", "", "l"),
	}, 0)
	require.NoError(t, err)
	require.Equal(t, [][]quad.Quad{{
		quad.MakeIRI("c", "p", "d", "l"),
		quad.MakeIRI("g", "p", "h", "l"),
	}}, results)

	// A value no link holds matches nothing.
	results, err = qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("", "missing", "", ""),
	}, 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Empty(t, results[0])
}

func TestQuadIteratorByPredicate(t *testing.T) {
	// Build the store.
	ctx := context.Background()
	qs := newPostingScanStore(ctx, t)

	// Iterate the links by predicate.
	it := qs.QuadIterator(ctx, quad.Predicate, mustValueOf(ctx, t, qs, quad.IRI("p"))).Iterate(ctx)
	defer it.Close()
	var got []quad.Quad
	for it.Next(ctx) {
		ref, err := it.Result(ctx)
		require.NoError(t, err)
		q, err := qs.Quad(ctx, ref)
		require.NoError(t, err)
		got = append(got, q)
	}

	// The live links come back in the order they were added.
	require.NoError(t, it.Err())
	require.Equal(t, []quad.Quad{
		quad.MakeIRI("a", "p", "b", ""),
		quad.MakeIRI("c", "p", "d", "l"),
		quad.MakeIRI("g", "p", "h", "l"),
		quad.MakeIRI("i", "p", "j", ""),
	}, got)
}
