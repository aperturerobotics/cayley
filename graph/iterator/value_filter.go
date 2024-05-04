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

import (
	"context"

	"github.com/cayleygraph/cayley/graph/refs"
	"github.com/cayleygraph/cayley/quad"
)

type ValueFilterFunc func(quad.Value) (bool, error)

type ValueFilter struct {
	sub    Shape
	filter ValueFilterFunc
	qs     refs.Namer
}

func NewValueFilter(qs refs.Namer, sub Shape, filter ValueFilterFunc) *ValueFilter {
	return &ValueFilter{
		sub:    sub,
		qs:     qs,
		filter: filter,
	}
}

func (it *ValueFilter) Iterate(ctx context.Context) Scanner {
	return newValueFilterNext(it.qs, it.sub.Iterate(ctx), it.filter)
}

func (it *ValueFilter) Lookup(ctx context.Context) Index {
	return newValueFilterContains(it.qs, it.sub.Lookup(ctx), it.filter)
}

func (it *ValueFilter) SubIterators() []Shape {
	return []Shape{it.sub}
}

func (it *ValueFilter) String() string {
	return "ValueFilter"
}

// There's nothing to optimize, locally, for a value-comparison iterator.
// Replace the underlying iterator if need be.
// potentially replace it.
func (it *ValueFilter) Optimize(ctx context.Context) (Shape, bool, error) {
	newSub, changed, err := it.sub.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if changed {
		it.sub = newSub
	}
	return it, true, nil
}

// We're only as expensive as our subiterator.
// Again, optimized value comparison iterators should do better.
func (it *ValueFilter) Stats(ctx context.Context) (Costs, error) {
	st, err := it.sub.Stats(ctx)
	st.Size.Value = st.Size.Value/2 + 1
	st.Size.Exact = false
	return st, err
}

type valueFilterNext struct {
	sub    Scanner
	filter ValueFilterFunc
	qs     refs.Namer
	result refs.Ref
	err    error
}

func newValueFilterNext(qs refs.Namer, sub Scanner, filter ValueFilterFunc) *valueFilterNext {
	return &valueFilterNext{
		sub:    sub,
		qs:     qs,
		filter: filter,
	}
}

func (it *valueFilterNext) doFilter(ctx context.Context, val refs.Ref) bool {
	qval, err := it.qs.NameOf(ctx, val)
	if err != nil {
		it.err = err
		return false
	}
	ok, err := it.filter(qval)
	if err != nil {
		it.err = err
	}
	return ok
}

func (it *valueFilterNext) Close() error {
	return it.sub.Close()
}

func (it *valueFilterNext) Next(ctx context.Context) bool {
	if it.Err() != nil {
		return false
	}
	for it.sub.Next(ctx) {
		val, err := it.sub.Result(ctx)
		if err != nil {
			it.err = err
			return false
		}
		if it.doFilter(ctx, val) {
			it.result = val
			return true
		}
	}
	it.err = it.sub.Err()
	return false
}

func (it *valueFilterNext) Err() error {
	return it.err
}

func (it *valueFilterNext) Result(ctx context.Context) (refs.Ref, error) {
	return it.result, it.Err()
}

func (it *valueFilterNext) NextPath(ctx context.Context) bool {
	return it.sub.NextPath(ctx)
}

// If we failed the check, then the subiterator should not contribute to the result
// set. Otherwise, go ahead and tag it.
func (it *valueFilterNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.sub.TagResults(ctx, dst)
}

func (it *valueFilterNext) String() string {
	return "ValueFilterNext"
}

type valueFilterContains struct {
	sub    Index
	filter ValueFilterFunc
	qs     refs.Namer
	result refs.Ref
	err    error
}

func newValueFilterContains(qs refs.Namer, sub Index, filter ValueFilterFunc) *valueFilterContains {
	return &valueFilterContains{
		sub:    sub,
		qs:     qs,
		filter: filter,
	}
}

func (it *valueFilterContains) doFilter(ctx context.Context, val refs.Ref) bool {
	qval, err := it.qs.NameOf(ctx, val)
	if err != nil {
		it.err = err
		return false
	}
	ok, err := it.filter(qval)
	if err != nil {
		it.err = err
	}
	return ok
}

func (it *valueFilterContains) Close() error {
	return it.sub.Close()
}

func (it *valueFilterContains) Err() error {
	return it.err
}

func (it *valueFilterContains) Result(ctx context.Context) (refs.Ref, error) {
	return it.result, it.err
}

func (it *valueFilterContains) NextPath(ctx context.Context) bool {
	return it.sub.NextPath(ctx)
}

func (it *valueFilterContains) Contains(ctx context.Context, val refs.Ref) (bool, error) {
	if !it.doFilter(ctx, val) {
		return false, it.err
	}
	ok, err := it.sub.Contains(ctx, val)
	if err != nil {
		return false, err
	}
	if !ok {
		it.err = it.sub.Err()
	}
	return ok, it.err
}

// If we failed the check, then the subiterator should not contribute to the result
// set. Otherwise, go ahead and tag it.
func (it *valueFilterContains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.sub.TagResults(ctx, dst)
}

func (it *valueFilterContains) String() string {
	return "ValueFilterContains"
}
