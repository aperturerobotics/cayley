package iterator

import (
	"context"
	"maps"
	"math"

	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

const recursiveBaseTag = "__base_recursive"

type seenAt struct {
	depth int
	tags  map[string]refs.Ref
	val   refs.Ref
}

var DefaultMaxRecursiveSteps = 50

// Recursive iterator takes a base iterator and a morphism to be applied recursively, for each result.
type Recursive struct {
	subIt     Shape
	morphism  Morphism
	maxDepth  int
	depthTags []string
}

func NewRecursive(it Shape, morphism Morphism, maxDepth int) *Recursive {
	if maxDepth == 0 {
		maxDepth = DefaultMaxRecursiveSteps
	}
	return &Recursive{
		subIt:    it,
		morphism: morphism,
		maxDepth: maxDepth,
	}
}

func (it *Recursive) Iterate(ctx context.Context) Scanner {
	return newRecursiveNext(it.subIt.Iterate(ctx), it.morphism, it.maxDepth, it.depthTags)
}

func (it *Recursive) Lookup(ctx context.Context) Index {
	return newRecursiveContains(newRecursiveNext(it.subIt.Iterate(ctx), it.morphism, it.maxDepth, it.depthTags))
}

func (it *Recursive) AddDepthTag(s string) {
	it.depthTags = append(it.depthTags, s)
}

func (it *Recursive) SubIterators() []Shape {
	return []Shape{it.subIt}
}

func (it *Recursive) Optimize(ctx context.Context) (Shape, bool, error) {
	newIt, optimized, err := it.subIt.Optimize(ctx)
	if err != nil {
		return it, false, err
	}
	if optimized {
		it.subIt = newIt
	}
	return it, false, nil
}

func (it *Recursive) Stats(ctx context.Context) (Costs, error) {
	base := NewFixed()
	base.Add(Int64Node(20))
	fanoutit := it.morphism(ctx, base)
	fanoutStats, err := fanoutit.Stats(ctx)
	subitStats, err2 := it.subIt.Stats(ctx)
	if err == nil {
		err = err2
	}
	size := int64(math.Pow(float64(subitStats.Size.Value*fanoutStats.Size.Value), 5))
	return Costs{
		NextCost:     subitStats.NextCost + fanoutStats.NextCost,
		ContainsCost: (subitStats.NextCost+fanoutStats.NextCost)*(size/10) + subitStats.ContainsCost,
		Size: refs.Size{
			Value: size,
			Exact: false,
		},
	}, err
}

func (it *Recursive) String() string {
	return "Recursive"
}

// Recursive iterator takes a base iterator and a morphism to be applied recursively, for each result.
type recursiveNext struct {
	subIt  Scanner
	result seenAt
	err    error

	morphism      Morphism
	seen          map[any]seenAt
	nextIt        Scanner
	depth         int
	maxDepth      int
	pathMap       map[any][]map[string]refs.Ref
	pathIndex     int
	containsValue refs.Ref
	depthTags     []string
	depthCache    []refs.Ref
	baseIt        *Fixed
}

func newRecursiveNext(it Scanner, morphism Morphism, maxDepth int, depthTags []string) *recursiveNext {
	return &recursiveNext{
		subIt:     it,
		morphism:  morphism,
		maxDepth:  maxDepth,
		depthTags: depthTags,

		seen:    make(map[any]seenAt),
		nextIt:  &Null{},
		baseIt:  NewFixed(),
		pathMap: make(map[any][]map[string]refs.Ref),
	}
}

func (it *recursiveNext) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	if err := it.Err(); err != nil {
		return err
	}

	for _, tag := range it.depthTags {
		dst[tag] = refs.PreFetched(quad.Int(it.result.depth))
	}

	if it.containsValue != nil {
		paths := it.pathMap[refs.ToKey(it.containsValue)]
		if len(paths) != 0 {
			maps.Copy(dst, paths[it.pathIndex])
		}
	}
	if it.nextIt != nil {
		err := it.nextIt.TagResults(ctx, dst)
		delete(dst, recursiveBaseTag)
		if err != nil {
			it.err = err
			return err
		}
	}
	return nil
}

func (it *recursiveNext) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	it.pathIndex = 0
	if it.depth == 0 {
		for it.subIt.Next(ctx) {
			res, err := it.subIt.Result(ctx)
			if err != nil {
				_ = it.subIt.Close()
				it.err = err
				return false
			}
			it.depthCache = append(it.depthCache, res)
			tags := make(map[string]refs.Ref)
			err = it.subIt.TagResults(ctx, tags)
			if err != nil {
				_ = it.subIt.Close()
				it.err = err
				return false
			}
			key := refs.ToKey(res)
			it.pathMap[key] = append(it.pathMap[key], tags)
			for it.subIt.NextPath(ctx) {
				tags := make(map[string]refs.Ref)
				err = it.subIt.TagResults(ctx, tags)
				if err != nil {
					_ = it.subIt.Close()
					it.err = err
					return false
				}
				it.pathMap[key] = append(it.pathMap[key], tags)
			}
		}
		if err := it.subIt.Err(); err != nil {
			_ = it.subIt.Close()
			it.err = err
			return false
		}
	}

	for {
		if !it.nextIt.Next(ctx) {
			if it.maxDepth > 0 && it.depth >= it.maxDepth {
				return false
			} else if len(it.depthCache) == 0 {
				return false
			}
			it.depth++
			it.baseIt = NewFixed(it.depthCache...)
			it.depthCache = nil
			if it.nextIt != nil {
				it.nextIt.Close()
			}
			it.nextIt = it.morphism(ctx, Tag(it.baseIt, recursiveBaseTag)).Iterate(ctx)
			continue
		}
		if err := it.nextIt.Err(); err != nil {
			it.err = err
			return false
		}
		val, err := it.nextIt.Result(ctx)
		if err != nil {
			it.err = err
			return false
		}
		results := make(map[string]refs.Ref)
		if err := it.nextIt.TagResults(ctx, results); err != nil {
			it.err = err
			return false
		}
		key := refs.ToKey(val)
		if _, seen := it.seen[key]; !seen {
			base := results[recursiveBaseTag]
			delete(results, recursiveBaseTag)
			it.seen[key] = seenAt{
				val:   base,
				depth: it.depth,
				tags:  results,
			}
			it.result.depth = it.depth
			it.result.val = val
			it.containsValue = it.getBaseValue(val)
			it.depthCache = append(it.depthCache, val)
			return true
		}
	}
}

func (it *recursiveNext) Err() error {
	return it.err
}

func (it *recursiveNext) Result(ctx context.Context) (refs.Ref, error) {
	return it.result.val, it.err
}

func (it *recursiveNext) getBaseValue(val refs.Ref) refs.Ref {
	var at seenAt
	var ok bool
	if at, ok = it.seen[refs.ToKey(val)]; !ok {
		panic("trying to getBaseValue of something unseen")
	}
	for at.depth != 1 {
		if at.depth == 0 {
			panic("seen chain is broken")
		}
		at = it.seen[refs.ToKey(at.val)]
	}
	return at.val
}

func (it *recursiveNext) NextPath(ctx context.Context) bool {
	if it.pathIndex+1 >= len(it.pathMap[refs.ToKey(it.containsValue)]) {
		return false
	}
	it.pathIndex++
	return true
}

func (it *recursiveNext) Close() error {
	err := it.subIt.Close()
	if err != nil {
		return err
	}
	err = it.nextIt.Close()
	if err != nil {
		return err
	}
	it.seen = nil
	return it.err
}

func (it *recursiveNext) String() string {
	return "RecursiveNext"
}

// Recursive iterator takes a base iterator and a morphism to be applied recursively, for each result.
type recursiveContains struct {
	next *recursiveNext
	tags map[string]refs.Ref
}

func newRecursiveContains(next *recursiveNext) *recursiveContains {
	return &recursiveContains{
		next: next,
	}
}

func (it *recursiveContains) TagResults(ctx context.Context, dst map[string]refs.Ref) error {
	if err := it.next.TagResults(ctx, dst); err != nil {
		return err
	}
	maps.Copy(dst, it.tags)
	return nil
}

func (it *recursiveContains) Err() error {
	return it.next.Err()
}

func (it *recursiveContains) Result(ctx context.Context) (refs.Ref, error) {
	return it.next.Result(ctx)
}

func (it *recursiveContains) Contains(ctx context.Context, val refs.Ref) (bool, error) {
	if err := it.next.Err(); err != nil {
		return false, err
	}
	it.next.pathIndex = 0
	key := refs.ToKey(val)
	if at, ok := it.next.seen[key]; ok {
		it.next.containsValue = it.next.getBaseValue(val)
		it.next.result.depth = at.depth
		it.next.result.val = val
		it.tags = at.tags
		return true, nil
	}
	for it.next.Next(ctx) {
		res, err := it.next.Result(ctx)
		if err != nil {
			return false, nil
		}
		if refs.ToKey(res) == key {
			return true, nil
		}
	}
	return false, nil
}

func (it *recursiveContains) NextPath(ctx context.Context) bool {
	return it.next.NextPath(ctx)
}

func (it *recursiveContains) Close() error {
	return it.next.Close()
}

func (it *recursiveContains) String() string {
	return "RecursiveContains(" + it.next.String() + ")"
}
