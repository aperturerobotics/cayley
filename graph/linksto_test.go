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

package graph_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/graphmock"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/quad"
)

func TestLinksTo(t *testing.T) {
	ctx := context.Background()
	object := quad.Raw("cool")
	q := quad.Quad{Subject: quad.IRI("alice"), Predicate: quad.IRI("is"), Object: object, Label: nil}
	qs := &graphmock.Store{
		Data: []quad.Quad{q},
	}
	fixed := iterator.NewFixed()

	val, err := qs.ValueOf(ctx, object)
	require.NoError(t, err)

	fixed.Add(val)
	lto := graph.NewLinksTo(qs, fixed, quad.Object).Iterate(ctx)
	require.True(t, lto.Next(ctx))
	res, err := lto.Result(ctx)
	require.NoError(t, err)
	qv, err := qs.Quad(ctx, res)
	require.NoError(t, err)
	require.Equal(t, q, qv)
}
