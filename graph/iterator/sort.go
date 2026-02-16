package iterator

import (
	"context"
	"maps"
	"sort"

	"github.com/aperturerobotics/cayley/graph/refs"
)

// Sort iterator orders values from it's subiterator.
type Sort struct {
	namer refs.Namer
	subIt Shape
}

// NewSort creates a new Sort iterator.
// TODO(dennwc): This iterator must not be used inside And: it may be moved to a Contains branch and won't do anything.
//
//	We should make And/Intersect account for this.
func NewSort(namer refs.Namer, subIt Shape) *Sort {
	return &Sort{namer, subIt}
}

func (it *Sort) Iterate(ctx context.Context) Scanner {
	return newSortNext(it.namer, it.subIt.Iterate(ctx))
}

func (it *Sort) Lookup(ctx context.Context) Index {
	// TODO(dennwc): Lookup doesn't need any sorting. Using it this way is a bug in the optimizer.
	//               But instead of failing here, let still allow the query to execute. It won't be sorted,
	//               but it will work at least. Later consider changing returning an error here.
	return it.subIt.Lookup(ctx)
}

func (it *Sort) Optimize(ctx context.Context) (Shape, bool, error) {
	newIt, optimized, err := it.subIt.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if optimized {
		it.subIt = newIt
	}
	return it, false, nil
}

func (it *Sort) Stats(ctx context.Context) (Costs, error) {
	subStats, err := it.subIt.Stats(ctx)
	return Costs{
		// TODO(dennwc): better cost calculation; we probably need an InitCost defined in Costs
		NextCost:     subStats.NextCost * 2,
		ContainsCost: subStats.ContainsCost,
		Size: refs.Size{
			Value: subStats.Size.Value,
			Exact: true,
		},
	}, err
}

func (it *Sort) String() string {
	return "Sort"
}

// SubIterators returns a slice of the sub iterators.
func (it *Sort) SubIterators() []Shape {
	return []Shape{it.subIt}
}

type sortValue struct {
	result
	str   string
	paths []result
}
type sortByString []sortValue

func (v sortByString) Len() int { return len(v) }
func (v sortByString) Less(i, j int) bool {
	return v[i].str < v[j].str
}
func (v sortByString) Swap(i, j int) { v[i], v[j] = v[j], v[i] }

type sortNext struct {
	namer     refs.Namer
	subIt     Scanner
	ordered   sortByString
	result    result
	err       error
	index     int
	pathIndex int
}

func newSortNext(namer refs.Namer, subIt Scanner) *sortNext {
	return &sortNext{
		namer:     namer,
		subIt:     subIt,
		pathIndex: -1,
	}
}

func (it *sortNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	maps.Copy(dst, it.result.tags)
	return nil
}

func (it *sortNext) Err() error {
	return it.err
}

func (it *sortNext) Result(ctx context.Context) (refs.Ref, error) {
	return it.result.id, it.err
}

func (it *sortNext) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	if it.ordered == nil {
		v, err := getSortedValues(ctx, it.namer, it.subIt)
		it.ordered = v
		it.err = err
		if it.err != nil {
			return false
		}
	}
	if it.index >= len(it.ordered) {
		return false
	}
	it.pathIndex = -1
	it.result = it.ordered[it.index].result
	it.index++
	return true
}

func (it *sortNext) NextPath(ctx context.Context) bool {
	if it.index >= len(it.ordered) {
		return false
	}
	r := it.ordered[it.index]
	if it.pathIndex+1 >= len(r.paths) {
		return false
	}
	it.pathIndex++
	it.result = r.paths[it.pathIndex]
	return true
}

func (it *sortNext) Close() error {
	it.ordered = nil
	return it.subIt.Close()
}

func (it *sortNext) String() string {
	return "SortNext"
}

func getSortedValues(ctx context.Context, namer refs.Namer, it Scanner) (sortByString, error) {
	var v sortByString
	for it.Next(ctx) {
		id, err := it.Result(ctx)
		if err != nil {
			return nil, err
		}
		// TODO(dennwc): batch and use refs.ValuesOf
		name, err := namer.NameOf(ctx, id)
		if err != nil {
			return nil, err
		}
		str := name.String()
		tags := make(map[string]refs.Ref)
		if err := it.TagResults(ctx, tags); err != nil {
			return nil, err
		}
		val := sortValue{
			result: result{id, tags},
			str:    str,
		}
		for it.NextPath(ctx) {
			tags = make(map[string]refs.Ref)
			if err := it.TagResults(ctx, tags); err != nil {
				return nil, err
			}
			val.paths = append(val.paths, result{id, tags})
		}
		v = append(v, val)
	}
	if err := it.Err(); err != nil {
		return v, err
	}
	sort.Sort(v)
	return v, nil
}
