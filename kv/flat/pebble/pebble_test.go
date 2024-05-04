//go:build !386 && !arm

package pebble

import (
	"testing"

	"github.com/cayleygraph/cayley/kv/flat"
	"github.com/cayleygraph/cayley/kv/kvtest"
)

func TestPebble(t *testing.T) {
	kvtest.RunTestLocal(t, flat.UpgradeOpenPath(OpenPath), &kvtest.Options{
		NoTx: true,
	})
}
