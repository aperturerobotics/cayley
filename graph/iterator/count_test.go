package iterator

import (
	"context"
	"testing"

	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

func TestCount(t *testing.T) {
	fixed := NewFixed(
		refs.PreFetched(quad.String("a")),
		refs.PreFetched(quad.String("b")),
		refs.PreFetched(quad.String("c")),
		refs.PreFetched(quad.String("d")),
		refs.PreFetched(quad.String("e")),
	)
	its := NewCount(fixed, nil)

	ctx := context.Background()
	itn := its.Iterate(ctx)
	require.True(t, itn.Next(ctx))
	resi, err := itn.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, refs.PreFetched(quad.Int(5)), resi)
	require.False(t, itn.Next(ctx))

	itc := its.Lookup(ctx)
	tc1, err := itc.Contains(ctx, refs.PreFetched(quad.Int(5)))
	require.NoError(t, err)
	require.True(t, tc1)
	tc2, err := itc.Contains(ctx, refs.PreFetched(quad.Int(3)))
	require.NoError(t, err)
	require.False(t, tc2)

	fixed2 := NewFixed(
		refs.PreFetched(quad.String("b")),
		refs.PreFetched(quad.String("d")),
	)
	its = NewCount(NewAnd(fixed, fixed2), nil)

	itn = its.Iterate(ctx)
	require.True(t, itn.Next(ctx))
	resi, err = itn.Result(ctx)
	require.NoError(t, err)
	require.Equal(t, refs.PreFetched(quad.Int(2)), resi)
	require.False(t, itn.Next(ctx))

	itc = its.Lookup(ctx)
	tc1, err = itc.Contains(ctx, refs.PreFetched(quad.Int(5)))
	require.NoError(t, err)
	require.False(t, tc1)
	tc2, err = itc.Contains(ctx, refs.PreFetched(quad.Int(2)))
	require.NoError(t, err)
	require.True(t, tc2)
}
