package btree

import (
	"testing"

	"github.com/cayleygraph/cayley/kv"
	"github.com/cayleygraph/cayley/kv/flat"
	"github.com/cayleygraph/cayley/kv/kvtest"
)

func TestBtree(t *testing.T) {
	kvtest.RunTest(t, func(t testing.TB) kv.KV {
		return flat.Upgrade(New())
	}, &kvtest.Options{
		NoLocks: true,
		NoTx:    true,
	})
}
