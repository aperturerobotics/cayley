package iterator_test

import (
	"context"
	"fmt"

	. "github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
)

// A testing iterator that returns the given values for Next() and Err().
type testIterator struct {
	Shape

	NextVal bool
	ErrVal  error
}

func newTestIterator(next bool, err error) Shape {
	return &testIterator{
		Shape:   NewFixed(),
		NextVal: next,
		ErrVal:  err,
	}
}

func (it *testIterator) Iterate(ctx context.Context) Scanner {
	return &testIteratorNext{
		Scanner: it.Shape.Iterate(ctx),
		NextVal: it.NextVal,
		ErrVal:  it.ErrVal,
	}
}

func (it *testIterator) Lookup(ctx context.Context) Index {
	return &testIteratorContains{
		Index:   it.Shape.Lookup(ctx),
		NextVal: it.NextVal,
		ErrVal:  it.ErrVal,
	}
}

// A testing iterator that returns the given values for Next() and Err().
type testIteratorNext struct {
	Scanner

	NextVal bool
	ErrVal  error
}

func (it *testIteratorNext) Next(ctx context.Context) bool {
	return it.NextVal
}

func (it *testIteratorNext) Err() error {
	return it.ErrVal
}

// A testing iterator that returns the given values for Next() and Err().
type testIteratorContains struct {
	Index

	NextVal bool
	ErrVal  error
}

func (it *testIteratorContains) Contains(ctx context.Context, v refs.Ref) (bool, error) {
	return it.NextVal, it.Err()
}

func (it *testIteratorContains) Err() error {
	return it.ErrVal
}

type Int64Quad int64

func (v Int64Quad) Key() interface{} { return v }

func (Int64Quad) IsNode() bool { return false }

var _ Shape = &Int64{}

// An All iterator across a range of int64 values, from `max` to `min`.
type Int64 struct {
	node     bool
	max, min int64
}

func (it *Int64) Iterate(ctx context.Context) Scanner {
	return newInt64Next(it.min, it.max, it.node)
}

func (it *Int64) Lookup(ctx context.Context) Index {
	return newInt64Contains(it.min, it.max, it.node)
}

// Creates a new Int64 with the given range.
func newInt64(min, max int64, node bool) *Int64 {
	return &Int64{
		node: node,
		min:  min,
		max:  max,
	}
}

func (it *Int64) String() string {
	return fmt.Sprintf("Int64(%d-%d)", it.min, it.max)
}

// No sub-iterators.
func (it *Int64) SubIterators() []Shape {
	return nil
}

// The number of elements in an Int64 is the size of the range.
// The size is exact.
func (it *Int64) Size() (int64, bool) {
	sz := (it.max - it.min) + 1
	return sz, true
}

func valToInt64(v refs.Ref) int64 {
	if v, ok := v.(Int64Node); ok {
		return int64(v)
	}
	return int64(v.(Int64Quad))
}

// There's nothing to optimize about this little iterator.
func (it *Int64) Optimize(ctx context.Context) (Shape, bool, error) { return it, false, nil }

// Stats for an Int64 are simple. Super cheap to do any operation,
// and as big as the range.
func (it *Int64) Stats(ctx context.Context) (Costs, error) {
	s, exact := it.Size()
	return Costs{
		ContainsCost: 1,
		NextCost:     1,
		Size: refs.Size{
			Value: s,
			Exact: exact,
		},
	}, nil
}

// An All iterator across a range of int64 values, from `max` to `min`.
type int64Next struct {
	node     bool
	max, min int64
	at       int64
	result   int64
}

// Creates a new Int64 with the given range.
func newInt64Next(min, max int64, node bool) *int64Next {
	return &int64Next{
		node: node,
		min:  min,
		max:  max,
		at:   min,
	}
}

func (it *int64Next) Close() error {
	return nil
}

func (it *int64Next) TagResults(ctx context.Context, dst map[string]refs.Ref) error { return nil }

func (it *int64Next) String() string {
	return fmt.Sprintf("Int64(%d-%d)", it.min, it.max)
}

// Next() on an Int64 all iterator is a simple incrementing counter.
// Return the next integer, and mark it as the result.
func (it *int64Next) Next(ctx context.Context) bool {
	if it.at == -1 {
		return false
	}
	val := it.at
	it.at = it.at + 1
	if it.at > it.max {
		it.at = -1
	}
	it.result = val
	return true
}

func (it *int64Next) Err() error {
	return nil
}

func (it *int64Next) toValue(v int64) refs.Ref {
	if it.node {
		return Int64Node(v)
	}
	return Int64Quad(v)
}

func (it *int64Next) Result(ctx context.Context) (refs.Ref, error) {
	return it.toValue(it.result), it.Err()
}

func (it *int64Next) NextPath(ctx context.Context) bool {
	return false
}

// An All iterator across a range of int64 values, from `max` to `min`.
type int64Contains struct {
	node     bool
	max, min int64
	at       int64
	result   int64
}

// Creates a new Int64 with the given range.
func newInt64Contains(min, max int64, node bool) *int64Contains {
	return &int64Contains{
		node: node,
		min:  min,
		max:  max,
		at:   min,
	}
}

func (it *int64Contains) Close() error {
	return nil
}

func (it *int64Contains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.Err()
}

func (it *int64Contains) String() string {
	return fmt.Sprintf("Int64(%d-%d)", it.min, it.max)
}

func (it *int64Contains) Err() error {
	return nil
}

func (it *int64Contains) toValue(v int64) refs.Ref {
	if it.node {
		return Int64Node(v)
	}
	return Int64Quad(v)
}

func (it *int64Contains) Result(ctx context.Context) (refs.Ref, error) {
	return it.toValue(it.result), it.Err()
}

func (it *int64Contains) NextPath(ctx context.Context) bool {
	return false
}

// No sub-iterators.
func (it *int64Contains) SubIterators() []Shape {
	return nil
}

// Contains() for an Int64 is merely seeing if the passed value is
// within the range, assuming the value is an int64.
func (it *int64Contains) Contains(ctx context.Context, tsv refs.Ref) (bool, error) {
	if err := it.Err(); err != nil {
		return false, err
	}
	v := valToInt64(tsv)
	if it.min <= v && v <= it.max {
		it.result = v
		return true, nil
	}
	return false, nil
}
