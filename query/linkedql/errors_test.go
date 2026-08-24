package linkedql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatMultiError(t *testing.T) {
	err := formatMultiError([]error{errors.New("first"), errors.New("second")})
	require.EqualError(t, err, "could not parse PropertyPath: first; second")
}
