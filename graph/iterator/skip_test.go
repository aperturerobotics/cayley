package iterator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/cayleygraph/cayley/graph/iterator"
)

func TestSkipIteratorBasics(t *testing.T) {
	ctx := context.Background()
	allIt := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
		Int64Node(4),
		Int64Node(5),
	)

	u := NewSkip(allIt, 0)
	expectSz, _ := allIt.Stats(ctx)
	sz, _ := u.Stats(ctx)
	require.Equal(t, expectSz.Size.Value, sz.Size.Value)

	require.Equal(t, []int{1, 2, 3, 4, 5}, iterated(t, u))

	u = NewSkip(allIt, 3)
	expectSz.Size.Value = 2
	if sz, _ := u.Stats(ctx); sz.Size.Value != expectSz.Size.Value {
		t.Errorf("Failed to check Skip size: got:%v expected:%v", sz.Size, expectSz.Size)
	}
	require.Equal(t, []int{4, 5}, iterated(t, u))

	uc := u.Lookup(ctx)
	for _, v := range []int{1, 2, 3} {
		cnt, err := uc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.False(t, cnt)
	}
	for _, v := range []int{4, 5} {
		cnt, err := uc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}

	uc = u.Lookup(ctx)
	for _, v := range []int{5, 4, 3} {
		cnt, err := uc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.False(t, cnt)
	}
	for _, v := range []int{1, 2} {
		cnt, err := uc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}

	// TODO(dennwc): check with NextPath
}
