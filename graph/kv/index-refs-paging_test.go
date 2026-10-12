package kv

import (
	"io"
	"slices"
	"strconv"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/quad"
)

// TestIterateIndexPrefixNextRefsAfter checks stable, distinct subject pages.
func TestIterateIndexPrefixNextRefsAfter(t *testing.T) {
	// Create interleaved subjects and several quads for each matching subject.
	ctx := t.Context()
	store, err := New(ctx, btree.New(), graph.Options{OptAssumeDefaultIdx: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	// Create interleaved quads through the graph writer.
	qs := store.(*QuadStore)
	writer, err := graph.NewQuadWriter("single", qs, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	for i := range 90 {
		// Interleave another object while retaining the same predicate.
		object := quad.IRI("other")
		if i%2 == 0 {
			object = quad.IRI("type")
		}
		subject := quad.IRI("subject/" + strconv.Itoa(90-i))
		for _, label := range []quad.Value{nil, quad.IRI("label")} {
			if err := writer.AddQuad(ctx, quad.Quad{Subject: subject, Predicate: quad.IRI("p"), Object: object, Label: label}); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Record the index's complete order without making a key-order assumption.
	predicate, err := qs.ValueOf(ctx, quad.IRI("p"))
	if err != nil {
		t.Fatal(err)
	}
	object, err := qs.ValueOf(ctx, quad.IRI("type"))
	if err != nil {
		t.Fatal(err)
	}
	values := []uint64{uint64(object.(Int64Value)), uint64(predicate.(Int64Value))}
	var want []Int64Value
	err = qs.IterateIndexPrefixNextRefs(ctx, DefaultQuadIndexes[1], values, func(ref Int64Value, _ func() (bool, error)) error {
		want = append(want, ref)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Resume from each last subject, including across base62 digit boundaries.
	var got []Int64Value
	var after Int64Value
	for {
		// Collect one bounded page through the seek-capable index API.
		page := make([]Int64Value, 0, 7)
		err := qs.IterateIndexPrefixNextRefsAfter(ctx, DefaultQuadIndexes[1], values, after, func(ref Int64Value, hasLive func() (bool, error)) error {
			// Require the matching subject to retain a live quad.
			live, err := hasLive()
			if err != nil {
				return err
			}
			if !live {
				t.Fatal("matching subject has no live quad")
			}

			// Stop after this page's seven subjects.
			page = append(page, ref)
			if len(page) == 7 {
				return io.EOF
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// Stop at the end or retain this page's last reference as the cursor.
		if len(page) == 0 {
			break
		}
		got = append(got, page...)
		after = page[len(page)-1]
	}

	// Each matching subject appears once, in the original posting order.
	if len(want) != 45 || !slices.Equal(got, want) {
		t.Fatalf("paged refs = %v, want %v", got, want)
	}
}
