package iterator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/aperturerobotics/cayley/graph/iterator"
)

func TestNotIteratorBasics(t *testing.T) {
	ctx := context.Background()
	allIt := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
		Int64Node(4),
	)

	toComplementIt := NewFixed(
		Int64Node(2),
		Int64Node(4),
	)

	not := NewNot(toComplementIt, allIt)

	st, _ := not.Stats(ctx)
	require.Equal(t, int64(2), st.Size.Value)

	expect := []int{1, 3}
	for range 2 {
		require.Equal(t, expect, iterated(t, not))
	}

	nc := not.Lookup(ctx)
	for _, v := range []int{1, 3} {
		cnt, err := nc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}

	for _, v := range []int{2, 4} {
		cnt, err := nc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.False(t, cnt)
	}
}

func TestNotIteratorErr(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("unique")
	allIt := newTestIterator(false, wantErr)

	toComplementIt := NewFixed()

	not := NewNot(toComplementIt, allIt).Iterate(ctx)

	require.False(t, not.Next(ctx))
	require.Equal(t, wantErr, not.Err())
}
