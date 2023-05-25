package iterator

import (
	"context"
	"fmt"

	"github.com/cayleygraph/cayley/graph/refs"
)

// Limit iterator will stop iterating if certain a number of values were encountered.
// Zero and negative Limit values means no Limit.
type Limit struct {
	limit int64
	it    Shape
}

func NewLimit(it Shape, max int64) *Limit {
	return &Limit{
		limit: max,
		it:    it,
	}
}

func (it *Limit) Iterate(ctx context.Context) Scanner {
	return NewLimitNext(it.it.Iterate(ctx), it.limit)
}

func (it *Limit) Lookup(ctx context.Context) Index {
	return newLimitContains(it.it.Lookup(ctx), it.limit)
}

// SubIterators returns a slice of the sub iterators.
func (it *Limit) SubIterators() []Shape {
	return []Shape{it.it}
}

func (it *Limit) Optimize(ctx context.Context) (Shape, bool, error) {
	nit, optimized, err := it.it.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if it.limit <= 0 { // no Limit
		return nit, true, nil
	}
	it.it = nit
	return it, optimized, nil
}

func (it *Limit) Stats(ctx context.Context) (Costs, error) {
	st, err := it.it.Stats(ctx)
	if it.limit > 0 && st.Size.Value > it.limit {
		st.Size.Value = it.limit
	}
	return st, err
}

func (it *Limit) String() string {
	return fmt.Sprintf("Limit(%d)", it.limit)
}

// Limit iterator will stop iterating if certain a number of values were encountered.
// Zero and negative Limit values means no Limit.
type limitNext struct {
	limit int64
	count int64
	it    Scanner
}

func NewLimitNext(it Scanner, limit int64) Scanner {
	return &limitNext{
		limit: limit,
		it:    it,
	}
}

func (it *limitNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.it.TagResults(ctx, dst)
}

// Next advances the Limit iterator. It will stop iteration if Limit was reached.
func (it *limitNext) Next(ctx context.Context) bool {
	if it.limit > 0 && it.count >= it.limit {
		return false
	}
	if it.it.Next(ctx) {
		it.count++
		return true
	}
	return false
}

func (it *limitNext) Err() error {
	return it.it.Err()
}

func (it *limitNext) Result(ctx context.Context) (refs.Ref, error) {
	return it.it.Result(ctx)
}

// NextPath checks whether there is another path. Will call primary iterator
// if Limit is not reached yet.
func (it *limitNext) NextPath(ctx context.Context) bool {
	if it.limit > 0 && it.count >= it.limit {
		return false
	}
	if it.it.NextPath(ctx) {
		it.count++
		return true
	}
	return false
}

// Close closes the primary and all iterators.  It closes all subiterators
// it can, but returns the first error it encounters.
func (it *limitNext) Close() error {
	return it.it.Close()
}

func (it *limitNext) String() string {
	return fmt.Sprintf("LimitNext(%d)", it.limit)
}

// Limit iterator will stop iterating if certain a number of values were encountered.
// Zero and negative Limit values means no Limit.
type limitContains struct {
	limit int64
	count int64
	it    Index
}

func newLimitContains(it Index, limit int64) *limitContains {
	return &limitContains{
		limit: limit,
		it:    it,
	}
}

func (it *limitContains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.it.TagResults(ctx, dst)
}

func (it *limitContains) Err() error {
	return it.it.Err()
}

func (it *limitContains) Result(ctx context.Context) (refs.Ref, error) {
	return it.it.Result(ctx)
}

func (it *limitContains) Contains(ctx context.Context, val refs.Ref) (bool, error) {
	if it.limit > 0 && it.count >= it.limit {
		return false, nil
	}
	cnt, err := it.it.Contains(ctx, val)
	if err != nil {
		return false, err
	}
	if cnt {
		it.count++
		return true, nil
	}
	return false, nil
}

// NextPath checks whether there is another path. Will call primary iterator
// if Limit is not reached yet.
func (it *limitContains) NextPath(ctx context.Context) bool {
	if it.limit > 0 && it.count >= it.limit {
		return false
	}
	if it.it.NextPath(ctx) {
		it.count++
		return true
	}
	return false
}

// Close closes the primary and all iterators.  It closes all subiterators
// it can, but returns the first error it encounters.
func (it *limitContains) Close() error {
	return it.it.Close()
}

func (it *limitContains) String() string {
	return fmt.Sprintf("LimitContains(%d)", it.limit)
}
