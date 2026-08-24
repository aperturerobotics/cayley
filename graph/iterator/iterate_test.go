package iterator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

type countingNamer struct {
	calls int
}

func (n *countingNamer) ValueOf(context.Context, quad.Value) (refs.Ref, error) {
	return nil, nil
}

func (n *countingNamer) NameOf(context.Context, refs.Ref) (quad.Value, error) {
	n.calls++
	return quad.Raw("value"), nil
}

func TestSendValuesNamesEachResultOnce(t *testing.T) {
	namer := new(countingNamer)
	out := make(chan quad.Value, 1)
	shape := NewFixed(refs.PreFetched(quad.Raw("result")))
	err := Iterate(shape).UnOptimized().SendValues(context.Background(), namer, out)
	require.NoError(t, err)
	require.Equal(t, 1, namer.calls)
	require.Equal(t, quad.Raw("value"), <-out)
}

func TestTagValuesReturnsCallbackError(t *testing.T) {
	namer := new(countingNamer)
	wantErr := errors.New("callback failed")
	shape := NewFixed(refs.PreFetched(quad.Raw("result")))
	err := Iterate(shape).UnOptimized().TagValues(context.Background(), namer, func(map[string]quad.Value) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}
