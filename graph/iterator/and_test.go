// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iterator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
)

// Make sure that tags work on the And.
func TestAndTag(t *testing.T) {
	ctx := context.Background()
	fix1 := NewFixed(Int64Node(234))
	fix2 := NewFixed(Int64Node(234))
	var ands Shape = NewAnd(Tag(fix1, "foo")).AddOptionalIterator(Tag(fix2, "baz"))
	ands = Tag(ands, "bar")

	and := ands.Iterate(ctx)
	require.True(t, and.Next(ctx))
	resi, err := and.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(234), resi)

	tags := make(map[string]refs.Ref)
	err = and.TagResults(ctx, tags)
	require.NoError(t, err)
	require.Equal(t, map[string]refs.Ref{
		"foo": Int64Node(234),
		"bar": Int64Node(234),
		"baz": Int64Node(234),
	}, tags)
}

// Do a simple itersection of fixed values.
func TestAndAndFixedIterators(t *testing.T) {
	ctx := context.Background()
	fix1 := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
		Int64Node(4),
	)
	fix2 := NewFixed(
		Int64Node(3),
		Int64Node(4),
		Int64Node(5),
	)
	ands := NewAnd(fix1, fix2)
	// Should be as big as smallest subiterator
	st, err := ands.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, refs.Size{
		Value: 3,
		Exact: true,
	}, st.Size)

	and := ands.Iterate(ctx)

	require.True(t, and.Next(ctx))
	resi, err := and.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(3), resi)

	require.True(t, and.Next(ctx))
	resi, err = and.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(4), resi)

	require.False(t, and.Next(ctx))
}

// If there's no intersection, the size should still report the same,
// but there should be nothing to Next()
func TestNonOverlappingFixedIterators(t *testing.T) {
	ctx := context.Background()
	fix1 := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
		Int64Node(4),
	)
	fix2 := NewFixed(
		Int64Node(5),
		Int64Node(6),
		Int64Node(7),
	)
	ands := NewAnd(fix1, fix2)
	// Should be as big as smallest subiterator
	st, err := ands.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, refs.Size{
		Value: 3,
		Exact: true,
	}, st.Size)

	and := ands.Iterate(ctx)
	require.False(t, and.Next(ctx))
}

func TestAllIterators(t *testing.T) {
	ctx := context.Background()
	all1 := newInt64(1, 5, true)
	all2 := newInt64(4, 10, true)
	and := NewAnd(all2, all1).Iterate(ctx)

	require.True(t, and.Next(ctx))
	resi, err := and.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(4), resi)

	require.True(t, and.Next(ctx))
	resi, err = and.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(5), resi)

	require.False(t, and.Next(ctx))
}

func TestAndIteratorErr(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("unique")
	allErr := newTestIterator(false, wantErr)

	and := NewAnd(
		allErr,
		newInt64(1, 5, true),
	).Iterate(ctx)

	require.False(t, and.Next(ctx))
	require.Equal(t, wantErr, and.Err())
}

// multiPath is a shape holding one value reachable by n paths, each tagging
// its path number.
type multiPath struct {
	val  refs.Ref
	tag  string
	n    int
	size int64
}

func (s *multiPath) String() string                      { return "multiPath" }
func (s *multiPath) Iterate(ctx context.Context) Scanner { return &multiPathIt{s: s, path: -1} }
func (s *multiPath) Lookup(ctx context.Context) Index    { return &multiPathIt{s: s, path: -1} }
func (s *multiPath) Stats(ctx context.Context) (Costs, error) {
	size := max(s.size, 1)
	return Costs{NextCost: 1, ContainsCost: 1, Size: refs.Size{Value: size, Exact: s.size == 0}}, nil
}
func (s *multiPath) Optimize(ctx context.Context) (Shape, bool, error) { return s, false, nil }
func (s *multiPath) SubIterators() []Shape                             { return nil }

type multiPathIt struct {
	s    *multiPath
	path int
	done bool
}

func (it *multiPathIt) String() string { return "multiPath" }
func (it *multiPathIt) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	dst[it.s.tag] = Int64Node(it.path)
	return nil
}
func (it *multiPathIt) Result(ctx context.Context) (refs.Ref, error) { return it.s.val, nil }
func (it *multiPathIt) Next(ctx context.Context) bool {
	if it.done {
		return false
	}
	it.done, it.path = true, 0
	return true
}
func (it *multiPathIt) Contains(ctx context.Context, v refs.Ref) (bool, error) {
	if v != it.s.val {
		return false, nil
	}
	it.path = 0
	return true, nil
}
func (it *multiPathIt) NextPath(ctx context.Context) bool {
	if it.path+1 >= it.s.n {
		return false
	}
	it.path++
	return true
}
func (it *multiPathIt) Err() error   { return nil }
func (it *multiPathIt) Close() error { return nil }

// An And yields the product of its sides' paths, whichever side is primary.
func TestAndNextPathProduct(t *testing.T) {
	ctx := context.Background()
	v := Int64Node(7)
	for _, order := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}} {
		subs := []Shape{
			&multiPath{val: v, tag: "a", n: 2},
			&multiPath{val: v, tag: "b", n: 3},
			&multiPath{val: v, tag: "c", n: 4},
		}
		and := NewAnd()
		for _, i := range order {
			and.AddSubIterator(subs[i])
		}
		it := and.Iterate(ctx)
		seen := make(map[[3]refs.Ref]bool)
		for it.Next(ctx) {
			for {
				tags := make(map[string]refs.Ref)
				require.NoError(t, it.TagResults(ctx, tags))
				seen[[3]refs.Ref{tags["a"], tags["b"], tags["c"]}] = true
				if !it.NextPath(ctx) {
					break
				}
			}
		}
		require.NoError(t, it.Err())
		require.Len(t, seen, 2*3*4, "order %v", order)
	}
}

// An And picks its small side as primary even when the other side's size
// estimate makes the cost product overflow int64.
func TestAndOptimizeHugeEstimate(t *testing.T) {
	ctx := context.Background()
	v := Int64Node(7)
	small := &multiPath{val: v, tag: "small", n: 1, size: 10}
	for _, huge := range []int64{1 << 40, 1 << 62} {
		big := &multiPath{val: v, tag: "big", n: 1, size: huge}
		opt, _, err := NewAnd(big, small).Optimize(ctx)
		require.NoError(t, err)
		require.Same(t, Shape(small), opt.SubIterators()[0], "size %d", huge)
	}
}
