package pebble

import (
	"testing"

	"github.com/aperturerobotics/cayley/kv/flat"
	"github.com/aperturerobotics/cayley/kv/kvtest"
)

func TestPebble(t *testing.T) {
	kvtest.RunTestLocal(t, flat.UpgradeOpenPath(OpenPath), &kvtest.Options{
		NoTx: true,
	})
}
