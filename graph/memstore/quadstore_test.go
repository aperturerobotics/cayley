// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memstore

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphtest"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/refs"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
	"github.com/cayleygraph/cayley/writer"
)

// This is a simple test graph.
//
//	+---+                        +---+
//	| A |-------               ->| F |<--
//	+---+       \------>+---+-/  +---+   \--+---+
//	             ------>|#B#|      |        | E |
//	+---+-------/      >+---+      |        +---+
//	| C |             /            v
//	+---+           -/           +---+
//	  ----    +---+/             |#G#|
//	      \-->|#D#|------------->+---+
//	          +---+
var simpleGraph = []quad.Quad{
	quad.MakeRaw("A", "follows", "B", ""),
	quad.MakeRaw("C", "follows", "B", ""),
	quad.MakeRaw("C", "follows", "D", ""),
	quad.MakeRaw("D", "follows", "B", ""),
	quad.MakeRaw("B", "follows", "F", ""),
	quad.MakeRaw("F", "follows", "G", ""),
	quad.MakeRaw("D", "follows", "G", ""),
	quad.MakeRaw("E", "follows", "F", ""),
	quad.MakeRaw("B", "status", "cool", "status_graph"),
	quad.MakeRaw("D", "status", "cool", "status_graph"),
	quad.MakeRaw("G", "status", "cool", "status_graph"),
}

func makeTestStore(t *testing.T, data []quad.Quad) (*QuadStore, graph.QuadWriter, []pair) {
	ctx := context.Background()
	seen := make(map[string]struct{})
	qs := New()
	var (
		val int64
		ind []pair
	)
	writer, _ := writer.NewSingleReplication(qs, nil)
	for _, td := range data {
		for _, dir := range quad.Directions {
			qp := td.GetString(dir)
			if _, ok := seen[qp]; !ok && qp != "" {
				val++
				ind = append(ind, pair{qp, val})
				seen[qp] = struct{}{}
			}
		}

		err := writer.AddQuad(ctx, td)
		require.NoError(t, err)
		val++
	}
	return qs, writer, ind
}

func TestMemstore(t *testing.T) {
	graphtest.TestAll(t, func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
		return New(), nil, func() {}
	}, &graphtest.Config{
		AlwaysRunIntegration: true,
	})
}

func BenchmarkMemstore(b *testing.B) {
	graphtest.BenchmarkAll(b, func(t testing.TB) (graph.QuadStore, graph.Options, func()) {
		return New(), nil, func() {}
	}, &graphtest.Config{
		AlwaysRunIntegration: true,
	})
}

type pair struct {
	query string
	value int64
}

func TestMemstoreValueOf(t *testing.T) {
	ctx := context.Background()
	qs, _, index := makeTestStore(t, simpleGraph)
	exp := graph.Stats{
		Nodes: refs.Size{Value: 11, Exact: true},
		Quads: refs.Size{Value: 11, Exact: true},
	}
	st, err := qs.Stats(ctx, true)
	require.NoError(t, err)
	require.Equal(t, exp, st, "Unexpected quadstore size")

	for _, test := range index {
		v, err := qs.ValueOf(ctx, quad.Raw(test.query))
		require.NoError(t, err)
		switch v := v.(type) {
		default:
			t.Errorf("ValueOf(%q) returned unexpected type, got:%T expected int64", test.query, v)
		case bnode:
			require.Equal(t, test.value, int64(v))
		}
	}
}

func TestIteratorsAndNextResultOrderA(t *testing.T) {
	ctx := context.Background()
	qs, _, _ := makeTestStore(t, simpleGraph)

	fixed := iterator.NewFixed()
	qsv, err := qs.ValueOf(ctx, quad.Raw("C"))
	require.NoError(t, err)
	fixed.Add(qsv)

	fixed2 := iterator.NewFixed()
	qsv, err = qs.ValueOf(ctx, quad.Raw("follows"))
	require.NoError(t, err)
	fixed2.Add(qsv)

	all := qs.NodesAllIterator(ctx)

	const allTag = "all"
	innerAnd := iterator.NewAnd(
		graph.NewLinksTo(qs, fixed2, quad.Predicate),
		graph.NewLinksTo(qs, iterator.Tag(all, allTag), quad.Object),
	)

	hasa := graph.NewHasA(qs, innerAnd, quad.Subject)
	outerAnd := iterator.NewAnd(fixed, hasa).Iterate(ctx)

	if !outerAnd.Next(ctx) {
		t.Error("Expected one matching subtree")
	}
	val, err := outerAnd.Result(ctx)
	require.NoError(t, err)
	vn, err := qs.NameOf(ctx, val)
	require.NoError(t, err)
	if vn != quad.Raw("C") {
		t.Errorf("Matching subtree should be %s, got %s", "barak", vn)
	}

	var (
		got    []string
		expect = []string{"B", "D"}
	)
	for {
		m := make(map[string]graph.Ref, 1)
		err := outerAnd.TagResults(ctx, m)
		require.NoError(t, err)
		mv, err := qs.NameOf(ctx, m[allTag])
		require.NoError(t, err)
		got = append(got, quad.ToString(mv))
		if !outerAnd.NextPath(ctx) {
			break
		}
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, expect) {
		t.Errorf("Unexpected result, got:%q expect:%q", got, expect)
	}

	if outerAnd.Next(ctx) {
		t.Error("More than one possible top level output?")
	}
}

func TestLinksToOptimization(t *testing.T) {
	qs, _, _ := makeTestStore(t, simpleGraph)

	ctx := context.Background()
	lto := shape.BuildIterator(ctx, qs, shape.Quads{
		{Dir: quad.Object, Values: shape.Lookup{quad.Raw("cool")}},
	})

	newIt, changed, err := lto.Optimize(ctx)
	require.NoError(t, err)
	if changed {
		t.Errorf("unexpected optimization step")
	}
	if _, ok := newIt.(*Iterator); !ok {
		t.Fatal("Didn't swap out to LLRB")
	}
}

func TestRemoveQuad(t *testing.T) {
	ctx := context.Background()
	qs, w, _ := makeTestStore(t, simpleGraph)

	err := w.RemoveQuad(ctx, quad.Make(
		"E",
		"follows",
		"F",
		nil,
	))

	if err != nil {
		t.Error("Couldn't remove quad", err)
	}

	fixed := iterator.NewFixed()
	qsv, err := qs.ValueOf(ctx, quad.Raw("E"))
	require.NoError(t, err)
	fixed.Add(qsv)

	fixed2 := iterator.NewFixed()
	qsv, err = qs.ValueOf(ctx, quad.Raw("follows"))
	require.NoError(t, err)
	fixed2.Add(qsv)

	innerAnd := iterator.NewAnd(
		graph.NewLinksTo(qs, fixed, quad.Subject),
		graph.NewLinksTo(qs, fixed2, quad.Predicate),
	)

	hasa := graph.NewHasA(qs, innerAnd, quad.Object)

	newIt, _, err := hasa.Optimize(ctx)
	require.NoError(t, err)
	if newIt.Iterate(ctx).Next(ctx) {
		t.Error("E should not have any followers.")
	}
}

func TestTransaction(t *testing.T) {
	qs, w, _ := makeTestStore(t, simpleGraph)
	ctx := context.Background()
	st, err := qs.Stats(ctx, true)
	require.NoError(t, err)

	tx := graph.NewTransaction()
	tx.AddQuad(quad.Make(
		"E",
		"follows",
		"G",
		nil))
	tx.RemoveQuad(quad.Make(
		"Non",
		"existent",
		"quad",
		nil))

	err = w.ApplyTransaction(ctx, tx)
	if err == nil {
		t.Error("Able to remove a non-existent quad")
	}
	st2, err := qs.Stats(context.Background(), true)
	require.NoError(t, err)
	require.Equal(t, st, st2, "Appended a new quad in a failed transaction")
}
