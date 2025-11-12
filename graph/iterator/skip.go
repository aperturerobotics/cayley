package iterator

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/cayley/graph/refs"
)

// Skip iterator will skip certain number of values from primary iterator.
type Skip struct {
	skip      int64
	primaryIt Shape
}

func NewSkip(primaryIt Shape, off int64) *Skip {
	return &Skip{
		skip:      off,
		primaryIt: primaryIt,
	}
}

func (it *Skip) Iterate(ctx context.Context) Scanner {
	return newSkipNext(it.primaryIt.Iterate(ctx), it.skip)
}

func (it *Skip) Lookup(ctx context.Context) Index {
	return newSkipContains(it.primaryIt.Lookup(ctx), it.skip)
}

// SubIterators returns a slice of the sub iterators.
func (it *Skip) SubIterators() []Shape {
	return []Shape{it.primaryIt}
}

func (it *Skip) Optimize(ctx context.Context) (Shape, bool, error) {
	optimizedPrimaryIt, optimized, err := it.primaryIt.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if it.skip == 0 { // nothing to skip
		return optimizedPrimaryIt, true, nil
	}
	it.primaryIt = optimizedPrimaryIt
	return it, optimized, nil
}

func (it *Skip) Stats(ctx context.Context) (Costs, error) {
	primaryStats, err := it.primaryIt.Stats(ctx)
	if primaryStats.Size.Exact {
		primaryStats.Size.Value -= it.skip
		if primaryStats.Size.Value < 0 {
			primaryStats.Size.Value = 0
		}
	}
	return primaryStats, err
}

func (it *Skip) String() string {
	return fmt.Sprintf("Skip(%d)", it.skip)
}

// Skip iterator will skip certain number of values from primary iterator.
type skipNext struct {
	skip      int64
	skipped   int64
	primaryIt Scanner
}

func newSkipNext(primaryIt Scanner, skip int64) *skipNext {
	return &skipNext{
		skip:      skip,
		primaryIt: primaryIt,
	}
}

func (it *skipNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.primaryIt.TagResults(ctx, dst)
}

// Next advances the Skip iterator. It will skip all initial values
// before returning actual result.
func (it *skipNext) Next(ctx context.Context) bool {
	for ; it.skipped < it.skip; it.skipped++ {
		if !it.primaryIt.Next(ctx) {
			return false
		}
	}
	return it.primaryIt.Next(ctx)
}

func (it *skipNext) Err() error {
	return it.primaryIt.Err()
}

func (it *skipNext) Result(ctx context.Context) (refs.Ref, error) {
	return it.primaryIt.Result(ctx)
}

// NextPath checks whether there is another path. It will skip first paths
// according to iterator parameter.
func (it *skipNext) NextPath(ctx context.Context) bool {
	for ; it.skipped < it.skip; it.skipped++ {
		if !it.primaryIt.NextPath(ctx) {
			return false
		}
	}
	return it.primaryIt.NextPath(ctx)
}

// Close closes the primary and all iterators.  It closes all subiterators
// it can, but returns the first error it encounters.
func (it *skipNext) Close() error {
	return it.primaryIt.Close()
}

func (it *skipNext) String() string {
	return fmt.Sprintf("SkipNext(%d)", it.skip)
}

// Skip iterator will skip certain number of values from primary iterator.
type skipContains struct {
	skip      int64
	skipped   int64
	primaryIt Index
}

func newSkipContains(primaryIt Index, skip int64) *skipContains {
	return &skipContains{
		skip:      skip,
		primaryIt: primaryIt,
	}
}

func (it *skipContains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	return it.primaryIt.TagResults(ctx, dst)
}

func (it *skipContains) Err() error {
	return it.primaryIt.Err()
}

func (it *skipContains) Result(ctx context.Context) (refs.Ref, error) {
	return it.primaryIt.Result(ctx)
}

func (it *skipContains) Contains(ctx context.Context, val refs.Ref) (bool, error) {
	inNextPath := false
	for it.skipped <= it.skip {
		// skipping main iterator results
		inNextPath = false
		cnt, err := it.primaryIt.Contains(ctx, val)
		if !cnt || err != nil {
			return false, err
		}
		it.skipped++

		// TODO(dennwc): we don't really know if we should call NextPath or not,
		//               and there is no good way to know
		if it.skipped <= it.skip {
			// skipping NextPath results
			inNextPath = true
			if !it.primaryIt.NextPath(ctx) {
				// main path exists, but we skipped it
				// and we skipped all alternative paths now
				// so we definitely "don't have" this value
				return false, it.primaryIt.Err()
			}
			it.skipped++

			for it.skipped <= it.skip {
				if !it.primaryIt.NextPath(ctx) {
					return false, it.primaryIt.Err()
				}
				it.skipped++
			}
		}
	}
	if inNextPath && it.primaryIt.NextPath(ctx) {
		if err := it.primaryIt.Err(); err != nil {
			return false, err
		}
		return true, nil
	}
	return it.primaryIt.Contains(ctx, val)
}

// NextPath checks whether there is another path. It will skip first paths
// according to iterator parameter.
func (it *skipContains) NextPath(ctx context.Context) bool {
	for ; it.skipped < it.skip; it.skipped++ {
		if !it.primaryIt.NextPath(ctx) {
			return false
		}
	}
	return it.primaryIt.NextPath(ctx)
}

// Close closes the primary and all iterators.  It closes all subiterators
// it can, but returns the first error it encounters.
func (it *skipContains) Close() error {
	return it.primaryIt.Close()
}

func (it *skipContains) String() string {
	return fmt.Sprintf("SkipContains(%d)", it.skip)
}
