package kv

import (
	"testing"

	"github.com/aperturerobotics/cayley/quad"
)

func TestIntersectSorted(t *testing.T) {
	tt := []struct {
		a      []uint64
		b      []uint64
		expect []uint64
	}{
		{
			a:      []uint64{1, 2, 3, 4, 5, 6},
			b:      []uint64{2, 4, 6, 8, 10},
			expect: []uint64{2, 4, 6},
		},
		{
			a:      []uint64{6, 7, 8, 9, 10, 11},
			b:      []uint64{1, 2, 3, 4, 5, 6},
			expect: []uint64{6},
		},
	}

	for i, x := range tt {
		c := intersectSortedUint64(x.a, x.b)
		if len(c) != len(x.expect) {
			t.Errorf("unexpected length: %d expected %d for test %d", len(c), len(x.expect), i)
		}
		for i, y := range c {
			if y != x.expect[i] {
				t.Errorf("unexpected entry: %#v expected %#v for test %d", c, x.expect, i)
			}
		}
	}
}

// TestPartialKeyPrefix verifies that a partial Key includes a trailing
// delimiter so prefix scans don't match unrelated IDs (e.g. ID 3 matching 32).
func TestPartialKeyPrefix(t *testing.T) {
	ind := QuadIndex{Dirs: []quad.Direction{quad.Subject, quad.Predicate}}

	full := ind.Key([]uint64{3, 2})
	partial := ind.Key([]uint64{3})
	other := ind.Key([]uint64{32, 2})

	// Partial key for ID 3 must match full key for (3, 2).
	if !full.HasPrefix(partial) {
		t.Fatal("full key (3,2) should have partial prefix (3)")
	}

	// Partial key for ID 3 must NOT match full key for (32, 2).
	if other.HasPrefix(partial) {
		t.Fatal("full key (32,2) should not have partial prefix (3)")
	}

}

func TestIndexlist(t *testing.T) {
	init := []uint64{5, 10, 2340, 32432, 3243366}
	b := appendIndex(nil, init)
	out, err := decodeIndex(b)
	if err != nil {
		t.Fatalf("couldn't decodeIndex: %s", err)
	}
	if len(out) != len(init) {
		t.Fatalf("mismatched lengths. got %#v expected %#v", out, init)
	}
	for i := range out {
		if out[i] != init[i] {
			t.Fatalf("mismatched element %d. got %#v expected %#v", i, out, init)
		}
	}
}
