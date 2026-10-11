package kv

import (
	"strings"
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

	// Recover the selected layout from metadata without supplying it again.
	reopened, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !CompareQuadIndexes(reopened.(*QuadStore).indexes.all, qs.indexes.all) {
		t.Fatal("new graph did not persist the explicit layout")
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
		[]QuadIndex{{}},
		[]QuadIndex{{Dirs: []quad.Direction{quad.Any}}},
	} {
		if _, err := New(t.Context(), db, graph.Options{OptQuadIndexes: indexes}); err == nil {
			t.Fatalf("accepted invalid layout %v", indexes)
		}
	}
}

// TestExplicitQuadIndexesRequireRebuild rejects a populated legacy layout until reindexed.
func TestExplicitQuadIndexesRequireRebuild(t *testing.T) {
	// Populate a legacy graph without explicit index metadata.
	ctx := t.Context()
	db := btree.New()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	g, err := New(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.ApplyDeltas(ctx, []graph.Delta{{
		Quad: quad.MakeIRI("subject", "predicate", "object", ""), Action: graph.Add,
	}}, graph.IgnoreOpts{}); err != nil {
		t.Fatal(err)
	}
	indexes := []QuadIndex{
		{Dirs: []quad.Direction{quad.Subject, quad.Predicate, quad.Object}},
		{Dirs: []quad.Direction{quad.Object, quad.Predicate, quad.Subject}},
	}
	opts := graph.Options{OptQuadIndexes: indexes, OptAssumeDefaultIdx: true}

	// Require both layouts and the recovery action in the opening error.
	_, err = New(ctx, db, opts)
	if err == nil {
		t.Fatal("opened the legacy layout with per-edge indexes")
	}
	for _, detail := range []string{"[sp,ops]", "[spo,ops]", "offline reindex command"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("open error lacks %q: %v", detail, err)
		}
	}

	// Open the requested layout only after the offline rebuild succeeds.
	r, err := NewIndexRebuilder(db, indexes)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := r.Rebuild(ctx); err != nil || n != 1 {
		t.Fatalf("rebuild = %d, %v", n, err)
	}
	reopened, err := New(ctx, db, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !CompareQuadIndexes(reopened.(*QuadStore).indexes.all, indexes) {
		t.Fatal("reopened graph uses the wrong indexes")
	}
}
