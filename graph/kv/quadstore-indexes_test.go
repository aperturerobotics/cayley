package kv

import (
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/quad"
)

// TestExplicitQuadIndexes requires caller-selected layouts to be independent
// of the metadata default and the caller's direction slices.
func TestExplicitQuadIndexes(t *testing.T) {
	// Open a graph whose forward index contains the object direction.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	indexes := []QuadIndex{
		{Dirs: []quad.Direction{quad.Subject, quad.Predicate, quad.Object}},
		{Dirs: []quad.Direction{quad.Object, quad.Predicate, quad.Subject}},
	}
	g, err := New(ctx, db, graph.Options{OptQuadIndexes: indexes})
	if err != nil {
		t.Fatal(err)
	}

	// Keep the opened graph's layout stable after caller mutation.
	indexes[0].Dirs[0] = quad.Label
	qs := g.(*QuadStore)
	if qs.indexes.all[0].Dirs[0] != quad.Subject {
		t.Fatal("opened graph shares the caller's index layout")
	}
	if err := qs.ApplyDeltas(ctx, []graph.Delta{{
		Quad: quad.MakeIRI("subject", "predicate", "object", ""), Action: graph.Add,
	}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
}

// TestInvalidExplicitQuadIndexes rejects layouts that cannot index graph quads.
func TestInvalidExplicitQuadIndexes(t *testing.T) {
	// Check malformed option types, empty layouts, and invalid directions.
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, indexes := range []any{
		"spo",
		[]QuadIndex{},
		[]QuadIndex{{Dirs: []quad.Direction{quad.Any}}},
	} {
		if _, err := New(t.Context(), db, graph.Options{OptQuadIndexes: indexes}); err == nil {
			t.Fatalf("accepted invalid layout %v", indexes)
		}
	}
}
