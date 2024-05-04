package badger

import (
	"testing"

	"github.com/cayleygraph/cayley/kv/flat"
	"github.com/cayleygraph/cayley/kv/kvtest"
)

func TestBadger(t *testing.T) {
	kvtest.RunTestLocal(t, flat.UpgradeOpenPath(OpenPath), nil)
}
