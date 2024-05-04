package kvtest

import (
	"context"
	"reflect"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/graphtest"
	"github.com/aperturerobotics/cayley/graph/graphtest/testutil"
	"github.com/aperturerobotics/cayley/graph/kv"
	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/query/shape"
	"github.com/stretchr/testify/require"
)

type DatabaseFunc func(t testing.TB) (hkv.KV, graph.Options, func())

type Config struct {
	AlwaysRunIntegration bool
}

func (c Config) quadStore() *graphtest.Config {
	return &graphtest.Config{
		NoPrimitives:         true,
		AlwaysRunIntegration: c.AlwaysRunIntegration,
	}
}

func newQuadStoreFunc(gen DatabaseFunc, bloom bool) testutil.DatabaseFunc {
	return func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
		return newQuadStore(t, gen, bloom)
	}
}

func NewQuadStoreFunc(gen DatabaseFunc) testutil.DatabaseFunc {
	return newQuadStoreFunc(gen, true)
}

func newQuadStore(t testing.TB, gen DatabaseFunc, bloom bool) (graph.QuadStore, graph.Options, func()) {
	db, opt, closer := gen(t)
	if opt == nil {
		opt = make(graph.Options)
	}
	if bloom {
		opt[kv.OptBloom] = true
	}
	ctx := context.Background()
	err := kv.Init(ctx, db, opt)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "init failed", "%v", err)
	}
	kdb, err := kv.New(ctx, db, opt)
	if err != nil {
		db.Close()
		closer()
		require.Fail(t, "create failed", "%v", err)
	}
	return kdb, opt, func() {
		kdb.Close()
		closer()
	}
}

func NewQuadStore(t testing.TB, gen DatabaseFunc) (graph.QuadStore, graph.Options, func()) {
	return newQuadStore(t, gen, true)
}

func TestAll(t *testing.T, gen DatabaseFunc, conf *Config) {
	if conf == nil {
		conf = &Config{}
	}
	qsgen := NewQuadStoreFunc(gen)
	t.Run("qs", func(t *testing.T) {
		graphtest.TestAll(t, qsgen, conf.quadStore())
	})
	qsgenNoBloom := newQuadStoreFunc(gen, false)
	t.Run("qs-no-bloom", func(t *testing.T) {
		graphtest.TestAll(t, qsgenNoBloom, conf.quadStore())
	})
	t.Run("optimize", func(t *testing.T) {
		testOptimize(t, gen, conf)
	})
}

func testOptimize(t *testing.T, gen DatabaseFunc, _ *Config) {
	ctx := context.Background()
	qs, opts, closer := NewQuadStore(t, gen)
	defer closer()

	testutil.MakeWriter(t, qs, opts, graphtest.MakeQuadSet()...)

	// With an linksto-fixed pair
	lto := shape.BuildIterator(ctx, qs, shape.Quads{
		{Dir: quad.Object, Values: shape.Lookup{quad.Raw("F")}},
	})

	oldIt := shape.BuildIterator(ctx, qs, shape.Quads{
		{Dir: quad.Object, Values: shape.Lookup{quad.Raw("F")}},
	}).Iterate(ctx)
	defer oldIt.Close()
	newIts, ok, err := lto.Optimize(ctx)
	require.NoError(t, err)
	if ok {
		t.Errorf("unexpected optimization step")
	}
	if _, ok := newIts.(*kv.QuadIterator); !ok {
		t.Errorf("Optimized iterator type does not match original, got: %T", newIts)
	}
	newIt := newIts.Iterate(ctx)
	defer newIt.Close()

	newQuads := graphtest.IteratedQuadsNext(t, qs, newIt)
	oldQuads := graphtest.IteratedQuadsNext(t, qs, oldIt)
	if !reflect.DeepEqual(newQuads, oldQuads) {
		t.Errorf("Optimized iteration does not match original")
	}

	oldIt.Next(ctx)
	oldResults := make(map[string]graph.Ref)
	err = oldIt.TagResults(ctx, oldResults)
	require.NoError(t, err)
	newIt.Next(ctx)
	newResults := make(map[string]graph.Ref)
	err = newIt.TagResults(ctx, newResults)
	require.NoError(t, err)
	if !reflect.DeepEqual(newResults, oldResults) {
		t.Errorf("Discordant tag results, new:%v old:%v", newResults, oldResults)
	}
}

func BenchmarkAll(t *testing.B, gen DatabaseFunc, conf *Config) {
	if conf == nil {
		conf = &Config{}
	}
	qsgen := NewQuadStoreFunc(gen)
	t.Run("qs", func(t *testing.B) {
		graphtest.BenchmarkAll(t, qsgen, conf.quadStore())
	})
}
