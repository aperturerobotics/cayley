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

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/refs"
)

var _ iterator.Shape = (*allIterator)(nil)

type allIterator struct {
	qs    *QuadStore
	all   []*Primitive
	maxid int64 // id of last observed insert (prim id)
	nodes bool
}

func (qs *QuadStore) newAllIterator(nodes bool, maxid int64) *allIterator {
	return &allIterator{
		qs: qs, all: qs.cloneAll(), nodes: nodes,
		maxid: maxid,
	}
}

func (it *allIterator) Iterate(ctx context.Context) iterator.Scanner {
	return it.qs.newAllIteratorNext(it.nodes, it.maxid, it.all)
}

func (it *allIterator) Lookup(ctx context.Context) iterator.Index {
	return it.qs.newAllIteratorContains(it.nodes, it.maxid)
}

func (it *allIterator) SubIterators() []iterator.Shape { return nil }
func (it *allIterator) Optimize(ctx context.Context) (iterator.Shape, bool, error) {
	return it, false, nil
}

func (it *allIterator) String() string {
	return "MemStoreAll"
}

func (it *allIterator) Stats(ctx context.Context) (iterator.Costs, error) {
	return iterator.Costs{
		NextCost:     1,
		ContainsCost: 1,
		Size: refs.Size{
			// TODO(dennwc): use maxid?
			Value: int64(len(it.all)),
			Exact: true,
		},
	}, nil
}

func (p *Primitive) filter(isNode bool, maxid int64) bool {
	if p.ID > maxid {
		return false
	} else if isNode && p.Value != nil {
		return true
	} else if !isNode && !p.Quad.Zero() {
		return true
	}
	return false
}

type allIteratorNext struct {
	qs    *QuadStore
	all   []*Primitive
	maxid int64 // id of last observed insert (prim id)
	nodes bool

	i    int // index into qs.all
	cur  *Primitive
	done bool
}

func (qs *QuadStore) newAllIteratorNext(nodes bool, maxid int64, all []*Primitive) *allIteratorNext {
	return &allIteratorNext{
		qs: qs, all: all, nodes: nodes,
		i: -1, maxid: maxid,
	}
}

func (it *allIteratorNext) ok(p *Primitive) bool {
	return p.filter(it.nodes, it.maxid)
}

func (it *allIteratorNext) Next(ctx context.Context) bool {
	it.cur = nil
	if it.done {
		return false
	}
	all := it.all
	if it.i >= len(all) {
		it.done = true
		return false
	}
	it.i++
	for ; it.i < len(all); it.i++ {
		p := all[it.i]
		if p.ID > it.maxid {
			break
		}
		if it.ok(p) {
			it.cur = p
			return true
		}
	}
	it.done = true
	return false
}

func (it *allIteratorNext) Result(ctx context.Context) (graph.Ref, error) {
	if it.cur == nil {
		return nil, nil
	}
	if !it.cur.Quad.Zero() {
		return qprim{p: it.cur}, nil
	}
	return bnode(it.cur.ID), nil
}

func (it *allIteratorNext) Err() error { return nil }
func (it *allIteratorNext) Close() error {
	it.done = true
	it.all = nil
	return nil
}

func (it *allIteratorNext) TagResults(ctx context.Context, dst map[string]graph.Ref) error {
	return it.Err()
}

func (it *allIteratorNext) String() string {
	return "MemStoreAllNext"
}
func (it *allIteratorNext) NextPath(ctx context.Context) bool { return false }

type allIteratorContains struct {
	qs    *QuadStore
	maxid int64 // id of last observed insert (prim id)
	nodes bool

	cur  *Primitive
	done bool
}

func (qs *QuadStore) newAllIteratorContains(nodes bool, maxid int64) *allIteratorContains {
	return &allIteratorContains{
		qs: qs, nodes: nodes,
		maxid: maxid,
	}
}

func (it *allIteratorContains) ok(p *Primitive) bool {
	return p.filter(it.nodes, it.maxid)
}

func (it *allIteratorContains) Contains(ctx context.Context, v graph.Ref) (bool, error) {
	it.cur = nil
	if it.done {
		return false, nil
	}
	id, ok := asID(v)
	if !ok {
		return false, nil
	}
	p := it.qs.prim[id]
	if p.ID > it.maxid {
		return false, nil
	}
	if !it.ok(p) {
		return false, nil
	}
	it.cur = p
	return true, nil
}

func (it *allIteratorContains) Result(ctx context.Context) (graph.Ref, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	if it.cur == nil {
		return nil, nil
	}
	if !it.cur.Quad.Zero() {
		return qprim{p: it.cur}, nil
	}
	return bnode(it.cur.ID), nil
}

func (it *allIteratorContains) Err() error { return nil }

func (it *allIteratorContains) Close() error {
	it.done = true
	return nil
}

func (it *allIteratorContains) TagResults(ctx context.Context, dst map[string]graph.Ref) error {
	return nil
}

func (it *allIteratorContains) String() string {
	return "MemStoreAllContains"
}
func (it *allIteratorContains) NextPath(ctx context.Context) bool { return false }
