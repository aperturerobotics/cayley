package leveldb

import (
	"testing"

	"github.com/cayleygraph/cayley/kv/flat"
	"github.com/cayleygraph/cayley/kv/kvtest"
)

func TestLeveldb(t *testing.T) {
	kvtest.RunTestLocal(t, flat.UpgradeOpenPath(OpenPath), nil)
}
