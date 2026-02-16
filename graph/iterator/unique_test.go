package iterator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/aperturerobotics/cayley/graph/iterator"
)

func TestUniqueIteratorBasics(t *testing.T) {
	ctx := context.TODO()
	allIt := NewFixed(
		Int64Node(1),
		Int64Node(2),
		Int64Node(3),
		Int64Node(3),
		Int64Node(2),
	)

	u := NewUnique(allIt)

	expect := []int{1, 2, 3}
	for range 2 {
		require.Equal(t, expect, iterated(t, u))
	}

	uc := u.Lookup(ctx)
	for _, v := range []int{1, 2, 3} {
		cnt, err := uc.Contains(ctx, Int64Node(v))
		require.NoError(t, err)
		require.True(t, cnt)
	}
}
