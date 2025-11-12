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

package iterator

// A simple iterator that, when first called Contains() or Next() upon, materializes the whole subiterator, stores it locally, and responds. Essentially a cache.

import (
	"context"

	"github.com/aperturerobotics/cayley/clog"
	"github.com/aperturerobotics/cayley/graph/refs"
)

const MaterializeLimit = 1000

type result struct {
	id   refs.Ref
	tags map[string]refs.Ref
}

type Materialize struct {
	sub        Shape
	expectSize int64
}

func NewMaterialize(sub Shape) *Materialize {
	return NewMaterializeWithSize(sub, 0)
}

func NewMaterializeWithSize(sub Shape, size int64) *Materialize {
	return &Materialize{
		sub:        sub,
		expectSize: size,
	}
}

func (it *Materialize) Iterate(ctx context.Context) Scanner {
	return newMaterializeNext(ctx, it.sub)
}

func (it *Materialize) Lookup(ctx context.Context) Index {
	return newMaterializeContains(ctx, it.sub)
}

func (it *Materialize) String() string {
	return "Materialize"
}

func (it *Materialize) SubIterators() []Shape {
	return []Shape{it.sub}
}

func (it *Materialize) Optimize(ctx context.Context) (Shape, bool, error) {
	newSub, changed, err := it.sub.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if changed {
		it.sub = newSub
		if IsNull(it.sub) {
			return it.sub, true, nil
		}
	}
	return it, false, nil
}

// The entire point of Materialize is to amortize the cost by
// putting it all up front.
func (it *Materialize) Stats(ctx context.Context) (Costs, error) {
	overhead := int64(2)
	var size refs.Size
	subitStats, err := it.sub.Stats(ctx)
	if it.expectSize > 0 {
		size = refs.Size{Value: it.expectSize, Exact: false}
	} else {
		size = subitStats.Size
	}
	return Costs{
		ContainsCost: overhead * subitStats.NextCost,
		NextCost:     overhead * subitStats.NextCost,
		Size:         size,
	}, err
}

type materializeNext struct {
	sub  Shape
	next Scanner

	containsMap map[interface{}]int
	values      [][]result
	index       int
	subindex    int
	hasRun      bool
	aborted     bool
	err         error
}

func newMaterializeNext(ctx context.Context, sub Shape) *materializeNext {
	return &materializeNext{
		containsMap: make(map[interface{}]int),
		sub:         sub,
		next:        sub.Iterate(ctx),
		index:       -1,
	}
}

func (it *materializeNext) Close() error {
	it.containsMap = nil
	it.values = nil
	it.hasRun = false
	return it.next.Close()
}

func (it *materializeNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	if !it.hasRun {
		return it.err
	}
	if it.aborted {
		if err := it.next.TagResults(ctx, dst); err != nil {
			return err
		}
		return it.err
	}
	res, err := it.Result(ctx)
	if err != nil {
		return err
	}
	if res != nil {
		for tag, value := range it.values[it.index][it.subindex].tags {
			dst[tag] = value
		}
	}
	return nil
}

func (it *materializeNext) String() string {
	return "Materialize"
}

func (it *materializeNext) Result(ctx context.Context) (refs.Ref, error) {
	if it.aborted {
		return it.next.Result(ctx)
	}
	if len(it.values) == 0 {
		return nil, it.err
	}
	if it.index == -1 {
		return nil, it.err
	}
	if it.index >= len(it.values) {
		return nil, it.err
	}
	return it.values[it.index][it.subindex].id, nil
}

func (it *materializeNext) Next(ctx context.Context) bool {
	if !it.hasRun {
		it.materializeSet(ctx)
	}
	if it.err != nil {
		return false
	}
	if it.aborted {
		n := it.next.Next(ctx)
		it.err = it.next.Err()
		return n
	}

	it.index++
	it.subindex = 0
	return it.index < len(it.values)
}

func (it *materializeNext) Err() error {
	return it.err
}

func (it *materializeNext) NextPath(ctx context.Context) bool {
	if !it.hasRun {
		it.materializeSet(ctx)
	}
	if it.err != nil {
		return false
	}
	if it.aborted {
		return it.next.NextPath(ctx)
	}

	it.subindex++
	if it.subindex >= len(it.values[it.index]) {
		// Don't go off the end of the world
		it.subindex--
		return false
	}
	return true
}

func (it *materializeNext) materializeSet(ctx context.Context) {
	i := 0
	mn := 0
outer:
	for it.next.Next(ctx) {
		i++
		if i > MaterializeLimit {
			it.aborted = true
			break
		}
		id, err := it.next.Result(ctx)
		if err != nil {
			if it.err == nil {
				it.err = err
			}
			break
		}
		val := refs.ToKey(id)
		if _, ok := it.containsMap[val]; !ok {
			it.containsMap[val] = len(it.values)
			it.values = append(it.values, nil)
		}
		index := it.containsMap[val]
		tags := make(map[string]refs.Ref, mn)
		if err := it.next.TagResults(ctx, tags); err != nil {
			if it.err == nil {
				it.err = err
			}
			break
		}
		if n := len(tags); n > mn {
			mn = n
		}
		it.values[index] = append(it.values[index], result{id: id, tags: tags})
		for it.next.NextPath(ctx) {
			i++
			if i > MaterializeLimit {
				it.aborted = true
				break
			}
			tags := make(map[string]refs.Ref, mn)
			if err := it.next.TagResults(ctx, tags); err != nil {
				if it.err == nil {
					it.err = err
				}
				break outer
			}
			if n := len(tags); n > mn {
				mn = n
			}
			it.values[index] = append(it.values[index], result{id: id, tags: tags})
		}
	}
	if it.err == nil {
		it.err = it.next.Err()
	}
	if it.err == nil && it.aborted {
		if clog.V(2) {
			clog.Infof("Aborting subiterator")
		}
		it.values = nil
		it.containsMap = nil
		_ = it.next.Close()
		it.next = it.sub.Iterate(ctx)
	}
	it.hasRun = true
}

type materializeContains struct {
	next *materializeNext
	sub  Index // only set if aborted
}

func newMaterializeContains(ctx context.Context, sub Shape) *materializeContains {
	return &materializeContains{
		next: newMaterializeNext(ctx, sub),
	}
}

func (it *materializeContains) Close() error {
	err := it.next.Close()
	if it.sub != nil {
		if err2 := it.sub.Close(); err2 != nil && err == nil {
			err = err2
		}
	}
	return err
}

func (it *materializeContains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	if it.sub != nil {
		return it.sub.TagResults(ctx, dst)
	}
	return it.next.TagResults(ctx, dst)
}

func (it *materializeContains) String() string {
	return "MaterializeContains"
}

func (it *materializeContains) Result(ctx context.Context) (refs.Ref, error) {
	if it.sub != nil {
		return it.sub.Result(ctx)
	}
	return it.next.Result(ctx)
}

func (it *materializeContains) Err() error {
	if err := it.next.Err(); err != nil {
		return err
	} else if it.sub == nil {
		return nil
	}
	return it.sub.Err()
}

func (it *materializeContains) run(ctx context.Context) {
	it.next.materializeSet(ctx)
	if it.next.aborted {
		it.sub = it.next.sub.Lookup(ctx)
	}
}

func (it *materializeContains) Contains(ctx context.Context, v refs.Ref) (bool, error) {
	if !it.next.hasRun {
		it.run(ctx)
	}
	if err := it.Err(); err != nil {
		return false, err
	}
	if it.sub != nil {
		return it.sub.Contains(ctx, v)
	}
	key := refs.ToKey(v)
	if i, ok := it.next.containsMap[key]; ok {
		it.next.index = i
		it.next.subindex = 0
		return true, nil
	}
	return false, nil
}

func (it *materializeContains) NextPath(ctx context.Context) bool {
	if !it.next.hasRun {
		it.run(ctx)
	}
	if it.next.Err() != nil {
		return false
	}
	if it.sub != nil {
		return it.sub.NextPath(ctx)
	}
	return it.next.NextPath(ctx)
}
