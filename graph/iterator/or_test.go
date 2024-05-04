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

func iterated(t *testing.T, s Shape) []int {
	ctx := context.Background()
	var res []int
	it := s.Iterate(ctx)
	defer it.Close()
	for it.Next(ctx) {
		resi, err := it.Result(ctx)
		require.NoError(t, err)
		res = append(res, int(resi.(Int64Node)))
	}
	return res
}

func TestOrIteratorBasics(t *testing.T) {
	ctx := context.Background()
	or := NewOr()
	f1 := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
	)
	f2 := NewFixed(
		Int64Node(3),
		Int64Node(9),
		Int64Node(20),
		Int64Node(21),
	)
	or.AddSubIterator(f1)
	or.AddSubIterator(f2)

	st, _ := or.Stats(ctx)
	require.Equal(t, int64(7), st.Size.Value)

	expect := []int{1, 2, 3, 3, 9, 20, 21}
	for i := 0; i < 2; i++ {
		require.Equal(t, expect, iterated(t, or))
	}

	// Check that optimization works.
	optOr, _, err := or.Optimize(ctx)
	require.NoError(t, err)
	require.Equal(t, expect, iterated(t, optOr))

	orc := or.Lookup(ctx)
	for _, v := range []int{2, 3, 21} {
		cnt, err := orc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}

	for _, v := range []int{22, 5, 0} {
		cnt, err := orc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.False(t, cnt)
	}
}

func TestShortCircuitingOrBasics(t *testing.T) {
	ctx := context.Background()
	var or *Or

	f1 := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
	)
	f2 := NewFixed(
		Int64Node(3),
		Int64Node(9),
		Int64Node(20),
		Int64Node(21),
	)

	or = NewShortCircuitOr()
	or.AddSubIterator(f1)
	or.AddSubIterator(f2)
	st, _ := or.Stats(ctx)
	require.Equal(t, refs.Size{
		Value: 4,
		Exact: true,
	}, st.Size)

	// It should extract the first iterators' numbers.
	or = NewShortCircuitOr()
	or.AddSubIterator(f1)
	or.AddSubIterator(f2)
	expect := []int{1, 2, 3}
	for i := 0; i < 2; i++ {
		require.Equal(t, expect, iterated(t, or))
	}

	// Check optimization works.
	optOr, _, err := or.Optimize(ctx)
	require.NoError(t, err)
	require.Equal(t, expect, iterated(t, optOr))

	// Check that numbers in either iterator exist.
	or = NewShortCircuitOr()
	or.AddSubIterator(f1)
	or.AddSubIterator(f2)

	orc := or.Lookup(ctx)
	for _, v := range []int{2, 3, 21} {
		cnt, err := orc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}
	for _, v := range []int{22, 5, 0} {
		cnt, err := orc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.False(t, cnt)
	}

	// Check that it pulls the second iterator's numbers if the first is empty.
	or = NewShortCircuitOr()
	or.AddSubIterator(NewFixed())
	or.AddSubIterator(f2)

	expect = []int{3, 9, 20, 21}
	for i := 0; i < 2; i++ {
		require.Equal(t, expect, iterated(t, or))
	}
	// Check optimization works.
	optOr, _, err = or.Optimize(ctx)
	require.NoError(t, err)
	require.Equal(t, expect, iterated(t, optOr))
}

func TestOrIteratorErr(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("unique")
	orErr := newTestIterator(false, wantErr)

	fix1 := NewFixed(Int64Node(1))

	or := NewOr(
		fix1,
		orErr,
		newInt64(1, 5, true),
	).Iterate(ctx)

	require.True(t, or.Next(ctx))
	resi, err := or.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, Int64Node(1), resi)

	require.False(t, or.Next(ctx))
	require.Equal(t, wantErr, or.Err())
}

func TestShortCircuitOrIteratorErr(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("unique")
	orErr := newTestIterator(false, wantErr)

	or := NewOr(
		orErr,
		newInt64(1, 5, true),
	).Iterate(ctx)

	require.False(t, or.Next(ctx))
	require.Equal(t, wantErr, or.Err())
}
