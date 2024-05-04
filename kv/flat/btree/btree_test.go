package btree

import (
	"testing"

	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/flat"
	"github.com/aperturerobotics/cayley/kv/kvtest"
)

func TestBtree(t *testing.T) {
	kvtest.RunTest(t, func(t testing.TB) kv.KV {
		return flat.Upgrade(New())
	}, &kvtest.Options{
		NoLocks: true,
		NoTx:    true,
	})
}
