package badger

import (
	"testing"

	"github.com/aperturerobotics/cayley/kv/flat"
	"github.com/aperturerobotics/cayley/kv/kvtest"
)

func TestBadger(t *testing.T) {
	kvtest.RunTestLocal(t, flat.UpgradeOpenPath(OpenPath), nil)
}
