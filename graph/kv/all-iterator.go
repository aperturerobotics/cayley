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

package kv

import (
	"context"
	"errors"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

type constraint struct {
	dir quad.Direction
	val Int64Value
}

type allIterator struct {
	qs    *QuadStore
	nodes bool
	cons  *constraint
}

func (qs *QuadStore) newAllIterator(nodes bool, cons *constraint) *allIterator {
	if nodes && cons != nil {
		panic("cannot use a kv all iterator across nodes with a constraint")
	}
	return &allIterator{
		qs:    qs,
		nodes: nodes,
		cons:  cons,
	}
}

func (it *allIterator) Iterate(ctx context.Context) iterator.Scanner {
	return it.qs.newAllIteratorNext(it.nodes, it.cons)
}

func (it *allIterator) Lookup(ctx context.Context) iterator.Index {
	return it.qs.newAllIteratorContains(it.nodes, it.cons)
}

// No subiterators.
func (it *allIterator) SubIterators() []iterator.Shape {
	return nil
}

func (it *allIterator) String() string {
	return "KVAll"
}

func (it *allIterator) Sorted() bool { return false }

func (it *allIterator) Optimize(ctx context.Context) (iterator.Shape, bool, error) {
	return it, false, nil
}

func (it *allIterator) Stats(ctx context.Context) (iterator.Costs, error) {
	sz, err := it.qs.Size(ctx)
	return iterator.Costs{
		ContainsCost: 1,
		NextCost:     2,
		Size: refs.Size{
			Value: sz,
			Exact: false,
		},
	}, err
}

type allIteratorNext struct {
	nodes   bool
	id      uint64
	buf     []*proto.Primitive
	prim    *proto.Primitive
	horizon int64
	qs      *QuadStore
	err     error
	cons    *constraint
	// next yields the next batch of IDs to read, set at the first Next.
	next func() []uint64
}

func (qs *QuadStore) newAllIteratorNext(nodes bool, cons *constraint) *allIteratorNext {
	if nodes && cons != nil {
		panic("cannot use a kv all iterator across nodes with a constraint\n")
	}
	return &allIteratorNext{
		qs:      qs,
		nodes:   nodes,
		horizon: qs.horizon(context.TODO()),
		cons:    cons,
	}
}

func (it *allIteratorNext) TagResults(ctx context.Context, dst map[string]graph.Ref) error {
	return nil
}

func (it *allIteratorNext) Close() error {
	return nil
}

func (it *allIteratorNext) Err() error {
	return it.err
}

func (it *allIteratorNext) Result(ctx context.Context) (graph.Ref, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	if it.id > uint64(it.horizon) {
		return nil, nil
	}
	if it.nodes {
		return Int64Value(it.id), nil
	}
	if it.prim == nil {
		return nil, nil
	}
	return it.prim, nil
}

const nextBatch = 100

func (it *allIteratorNext) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	for {
		if len(it.buf) == 0 {
			if it.next == nil {
				if it.next, it.err = it.batches(ctx); it.err != nil {
					return false
				}
			}
			ids := it.next()
			if len(ids) == 0 {
				return false
			}
			it.buf, it.err = it.qs.getPrimitives(ctx, ids)
			if it.err != nil || len(it.buf) == 0 {
				return false
			}
		} else {
			it.buf = it.buf[1:]
		}
		for ; len(it.buf) > 0; it.buf = it.buf[1:] {
			p := it.buf[0]
			it.prim = p
			if p == nil || p.Deleted {
				continue
			}
			it.id = it.prim.ID
			if p.IsNode() && it.nodes {
				return true
			}
			if !p.IsNode() && !it.nodes {
				if it.cons == nil {
					return true
				}
				if Int64Value(p.GetDirection(it.cons.dir)) == it.cons.val {
					return true
				}
			}
		}
	}
}

// batches returns a function yielding the next batch of IDs to read in
// ascending order, empty once none remain. A constraint on a direction some
// index holds reads the IDs posted under its value; otherwise every ID up to
// the horizon is read.
func (it *allIteratorNext) batches(ctx context.Context) (func() []uint64, error) {
	// Read the posted IDs when an index holds the constrained direction.
	if it.cons != nil {
		values := map[quad.Direction]uint64{it.cons.dir: uint64(it.cons.val)}
		if ind, ok := it.qs.postingScanIndex(values); ok {
			tx, err := it.qs.db.Tx(ctx, false)
			if err != nil {
				return nil, err
			}
			defer tx.Close()
			ids, err := it.qs.postingLinkIDs(ctx, tx, ind, values)
			if err != nil {
				return nil, err
			}
			return func() []uint64 {
				batch := ids[:min(len(ids), nextBatch)]
				ids = ids[len(batch):]
				return batch
			}, nil
		}
	}

	// Otherwise walk every ID the log allocated.
	return func() []uint64 {
		ids := make([]uint64, 0, nextBatch)
		for len(ids) < nextBatch && it.id < uint64(it.horizon) {
			it.id++
			ids = append(ids, it.id)
		}
		return ids
	}, nil
}

func (it *allIteratorNext) NextPath(ctx context.Context) bool {
	return false
}

func (it *allIteratorNext) String() string {
	return "KVAllNext"
}

func (it *allIteratorNext) Sorted() bool { return false }

type allIteratorContains struct {
	nodes   bool
	id      uint64
	prim    *proto.Primitive
	horizon int64
	qs      *QuadStore
	err     error
	cons    *constraint
}

func (qs *QuadStore) newAllIteratorContains(nodes bool, cons *constraint) *allIteratorContains {
	if nodes && cons != nil {
		panic("cannot use a kv all iterator across nodes with a constraint")
	}
	return &allIteratorContains{
		qs:      qs,
		nodes:   nodes,
		horizon: qs.horizon(context.TODO()),
		cons:    cons,
	}
}

func (it *allIteratorContains) TagResults(ctx context.Context, dst map[string]graph.Ref) error {
	return nil
}

func (it *allIteratorContains) Close() error {
	return nil
}

func (it *allIteratorContains) Err() error {
	return it.err
}

func (it *allIteratorContains) Result(ctx context.Context) (graph.Ref, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	if it.id > uint64(it.horizon) {
		return nil, nil
	}
	if it.nodes {
		return Int64Value(it.id), nil
	}
	if it.prim == nil {
		return nil, nil
	}
	return it.prim, nil
}

func (it *allIteratorContains) NextPath(ctx context.Context) bool {
	return false
}

func (it *allIteratorContains) Contains(ctx context.Context, v graph.Ref) (bool, error) {
	// TODO(dennwc): This method doesn't check if the primitive still exists in the store.
	//               It's okay if we assume we provide the snapshot of data, though.
	//               However, passing a hand-crafted Ref will cause invalid results.
	//               Same is true for QuadIterator.
	if it.err != nil {
		return false, it.err
	}
	if it.nodes {
		x, ok := v.(Int64Value)
		if !ok {
			return false, nil
		}
		it.id = uint64(x)
		return it.id <= uint64(it.horizon), nil
	}
	p, ok := v.(*proto.Primitive)
	if !ok {
		return false, errors.New("ref must be a proto.Primitive")
	}
	it.prim = p
	it.id = it.prim.ID
	if it.cons == nil {
		return true, nil
	}
	if Int64Value(it.prim.GetDirection(it.cons.dir)) != it.cons.val {
		return false, nil
	}
	return true, nil
}

func (it *allIteratorContains) String() string {
	return "KVAllContains"
}

func (it *allIteratorContains) Sorted() bool { return false }
