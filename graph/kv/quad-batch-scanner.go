package kv

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/pquads"
)

// QuadFilterBatchCollector collects full quads for concrete quad filters.
type QuadFilterBatchCollector interface {
	CollectFilteredQuadsBatch(ctx context.Context, filters []quad.Quad, limitPerFilter uint32) ([][]quad.Quad, error)
}

// CollectFilteredQuadsBatch collects full quads for concrete quad filters.
func (qs *QuadStore) CollectFilteredQuadsBatch(ctx context.Context, filters []quad.Quad, limitPerFilter uint32) ([][]quad.Quad, error) {
	results := make([][]quad.Quad, len(filters))
	if len(filters) == 0 {
		return results, nil
	}

	tx, err := qs.db.Tx(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Close()

	resolved, err := qs.resolveQuadBatchFilters(ctx, tx, filters)
	if err != nil {
		return nil, err
	}

	groupsByKey := make(map[string]int)
	var groups []quadFilterScanGroup
	for i := range resolved {
		filter := &resolved[i]
		if !filter.valid {
			continue
		}
		if len(filter.vals) == 0 {
			if err := qs.collectAllQuadsForBatchFilter(ctx, tx, *filter, &results[i], limitPerFilter); err != nil {
				return nil, err
			}
			continue
		}
		if filter.index == nil {
			if err := qs.collectAllQuadsForBatchFilter(ctx, tx, *filter, &results[i], limitPerFilter); err != nil {
				return nil, err
			}
			continue
		}
		key := quadBatchScanKey(filter.index.Key(filter.vals))
		groupIdx, ok := groupsByKey[key]
		if !ok {
			groupIdx = len(groups)
			groupsByKey[key] = groupIdx
			groups = append(groups, quadFilterScanGroup{
				index: *filter.index,
				vals:  append([]uint64(nil), filter.vals...),
			})
		}
		groups[groupIdx].filterIndexes = append(groups[groupIdx].filterIndexes, i)
	}

	for _, group := range groups {
		if err := qs.collectQuadFilterScanGroup(ctx, tx, resolved, results, group, limitPerFilter); err != nil {
			return nil, err
		}
	}
	return results, nil
}

type resolvedQuadBatchFilter struct {
	index *QuadIndex
	all   map[quad.Direction]uint64
	vals  []uint64
	valid bool
}

type quadFilterScanGroup struct {
	index         QuadIndex
	vals          []uint64
	filterIndexes []int
}

func quadBatchScanKey(key kv.Key) string {
	var out []byte
	for _, part := range key {
		out = append(out, part...)
		out = append(out, 0)
	}
	return string(out)
}

type quadBatchValueRef struct {
	filter int
	dir    quad.Direction
	value  quad.Value
}

func (qs *QuadStore) resolveQuadBatchFilters(ctx context.Context, tx kv.Tx, filters []quad.Quad) ([]resolvedQuadBatchFilter, error) {
	var refs []quadBatchValueRef
	for filterIdx, filter := range filters {
		for _, dir := range quad.Directions {
			value := filter.Get(dir)
			if value == nil {
				continue
			}
			refs = append(refs, quadBatchValueRef{
				filter: filterIdx,
				dir:    dir,
				value:  value,
			})
		}
	}

	ids, err := qs.resolveQuadValuesForBatch(ctx, tx, refs)
	if err != nil {
		return nil, err
	}

	out := make([]resolvedQuadBatchFilter, len(filters))
	for i := range out {
		out[i].valid = true
	}
	for i, ref := range refs {
		id := ids[i]
		if id == 0 {
			out[ref.filter].valid = false
			continue
		}
		if out[ref.filter].all == nil {
			out[ref.filter].all = make(map[quad.Direction]uint64)
		}
		out[ref.filter].all[ref.dir] = id
	}

	for filterIdx := range out {
		if !out[filterIdx].valid || len(out[filterIdx].all) == 0 {
			continue
		}
		dirs := quadBatchScanDirs(out[filterIdx].all)
		indexes := qs.bestIndexes(dirs)
		if len(indexes) == 0 {
			continue
		}
		index := indexes[0]
		out[filterIdx].index = &index
		for _, dir := range index.Dirs {
			id, ok := out[filterIdx].all[dir]
			if !ok {
				break
			}
			out[filterIdx].vals = append(out[filterIdx].vals, id)
		}
	}
	return out, nil
}

func quadBatchScanDirs(values map[quad.Direction]uint64) []quad.Direction {
	if _, ok := values[quad.Subject]; ok {
		return []quad.Direction{quad.Subject}
	}
	if _, ok := values[quad.Object]; ok {
		return []quad.Direction{quad.Object}
	}
	dirs := make([]quad.Direction, 0, len(values))
	for dir := range values {
		dirs = append(dirs, dir)
	}
	return dirs
}

func (qs *QuadStore) resolveQuadValuesForBatch(ctx context.Context, tx kv.Tx, refs []quadBatchValueRef) ([]uint64, error) {
	vals := make([]quad.Value, len(refs))
	for i, ref := range refs {
		vals[i] = ref.value
	}
	return qs.resolveQuadValues(ctx, tx, vals)
}

// collectQuadFilterScanGroup collects the quads of the filters that share one
// index prefix. Each index entry under the prefix posts the IDs of one key,
// usually a single quad, so the scan gathers IDs across entries and fetches
// their primitives and values nextBatch at a time.
func (qs *QuadStore) collectQuadFilterScanGroup(
	ctx context.Context,
	tx kv.Tx,
	filters []resolvedQuadBatchFilter,
	results [][]quad.Quad,
	group quadFilterScanGroup,
	limit uint32,
) error {
	it := tx.Scan(ctx, options.WithPrefixKV(group.index.Key(group.vals)))
	defer it.Close()

	filled := make([]bool, len(filters))
	remaining := 0
	if limit != 0 {
		remaining = len(group.filterIndexes)
	}

	// flush matches the gathered IDs against the filters and appends their
	// quads. It reports whether every filter reached the limit.
	flush := func(ids []uint64) (bool, error) {
		prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
		if err != nil {
			return false, err
		}
		var matches []quadBatchPrimitiveMatch
		for _, prim := range prims {
			if prim == nil || prim.Deleted {
				continue
			}
			match := quadBatchPrimitiveMatch{prim: prim}
			for _, filterIdx := range group.filterIndexes {
				if filled[filterIdx] || !primitiveMatchesQuadFilter(prim, filters[filterIdx]) {
					continue
				}
				match.filterIndexes = append(match.filterIndexes, filterIdx)
			}
			if len(match.filterIndexes) != 0 {
				matches = append(matches, match)
			}
		}
		quads, err := qs.quadBatchPrimitiveMatchesToQuads(ctx, tx, matches)
		if err != nil {
			return false, err
		}
		for matchIdx, match := range matches {
			q := quads[matchIdx]
			for _, filterIdx := range match.filterIndexes {
				if filled[filterIdx] {
					continue
				}
				results[filterIdx] = append(results[filterIdx], q)
				if limit != 0 && uint32(len(results[filterIdx])) >= limit {
					filled[filterIdx] = true
					remaining--
				}
			}
			if limit != 0 && remaining == 0 {
				return true, nil
			}
		}
		return false, nil
	}

	// Gather IDs across index entries and flush each full batch.
	pending := make([]uint64, 0, nextBatch)
	for it.Next(ctx) {
		ids, err := decodeIndex(it.Val())
		if err != nil {
			return err
		}
		for len(ids) > 0 {
			n := min(len(ids), nextBatch-len(pending))
			pending, ids = append(pending, ids[:n]...), ids[n:]
			if len(pending) < nextBatch {
				continue
			}
			done, err := flush(pending)
			if err != nil || done {
				return err
			}
			pending = pending[:0]
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	if len(pending) != 0 {
		_, err := flush(pending)
		return err
	}
	return nil
}

// collectAllQuadsForBatchFilter collects the quads of a filter no index
// prefix serves, in primitive ID order. A filter with a value in a direction
// some index holds reads the IDs from that index's postings; any other filter
// walks every ID in the log.
func (qs *QuadStore) collectAllQuadsForBatchFilter(ctx context.Context, tx kv.Tx, filter resolvedQuadBatchFilter, results *[]quad.Quad, limit uint32) error {
	// Find the candidate IDs.
	next, err := qs.unindexedFilterIDs(ctx, tx, filter)
	if err != nil {
		return err
	}

	// Match their primitives in batches until the limit fills.
	for {
		ids := next()
		if len(ids) == 0 {
			return nil
		}
		prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
		if err != nil {
			return err
		}
		var matches []quadBatchPrimitiveMatch
		for _, prim := range prims {
			if prim == nil || prim.Deleted || prim.IsNode() || !primitiveMatchesQuadFilter(prim, filter) {
				continue
			}
			matches = append(matches, quadBatchPrimitiveMatch{prim: prim})
		}
		quads, err := qs.quadBatchPrimitiveMatchesToQuads(ctx, tx, matches)
		if err != nil {
			return err
		}
		for _, q := range quads {
			*results = append(*results, q)
			if limit != 0 && uint32(len(*results)) >= limit {
				return nil
			}
		}
	}
}

// unindexedFilterIDs returns a function yielding the next batch of candidate
// IDs for filter in ascending order, empty once none remain: the IDs posted
// under its values when an index holds one of its directions, otherwise every
// ID up to the horizon.
func (qs *QuadStore) unindexedFilterIDs(ctx context.Context, tx kv.Tx, filter resolvedQuadBatchFilter) (func() []uint64, error) {
	// Read the posted IDs when an index holds a filtered direction.
	if ind, ok := qs.postingScanIndex(filter.all); ok {
		ids, err := qs.postingLinkIDs(ctx, tx, ind, filter.all)
		if err != nil {
			return nil, err
		}
		return func() []uint64 {
			batch := ids[:min(len(ids), nextBatch)]
			ids = ids[len(batch):]
			return batch
		}, nil
	}

	// Otherwise walk every ID the log allocated.
	horizon, err := qs.getMetaIntTx(ctx, tx, "horizon")
	if err == kv.ErrNotFound {
		horizon = 0
	} else if err != nil {
		return nil, err
	}
	var id uint64
	return func() []uint64 {
		ids := make([]uint64, 0, nextBatch)
		for len(ids) < nextBatch && id < uint64(horizon) {
			id++
			ids = append(ids, id)
		}
		return ids
	}, nil
}

func primitiveMatchesQuadFilter(prim *proto.Primitive, filter resolvedQuadBatchFilter) bool {
	for dir, id := range filter.all {
		if prim.GetDirection(dir) != id {
			return false
		}
	}
	return true
}

type quadBatchPrimitiveMatch struct {
	prim          *proto.Primitive
	filterIndexes []int
}

func (qs *QuadStore) quadBatchPrimitiveMatchesToQuads(ctx context.Context, tx kv.Tx, matches []quadBatchPrimitiveMatch) ([]quad.Quad, error) {
	if len(matches) == 0 {
		return nil, nil
	}
	idsByValue := make(map[uint64]int)
	var values []quadBatchPrimitiveValue
	for matchIdx, match := range matches {
		for _, dir := range quad.Directions {
			id := match.prim.GetDirection(dir)
			if id == 0 {
				continue
			}
			ref := quadBatchPrimitiveValueRef{
				match: matchIdx,
				dir:   dir,
			}
			if valueIdx, ok := idsByValue[id]; ok {
				values[valueIdx].refs = append(values[valueIdx].refs, ref)
				continue
			}
			idsByValue[id] = len(values)
			values = append(values, quadBatchPrimitiveValue{
				id:   id,
				refs: []quadBatchPrimitiveValueRef{ref},
			})
		}
	}
	ids := make([]uint64, len(values))
	for i, value := range values {
		ids[i] = value.id
	}
	prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]quad.Quad, len(matches))
	for i, p := range prims {
		if p == nil {
			continue
		}
		value, err := pquads.UnmarshalValue(ctx, p.Value)
		if err != nil {
			return out, err
		}
		for _, ref := range values[i].refs {
			out[ref.match].Set(ref.dir, value)
		}
	}
	return out, nil
}

type quadBatchPrimitiveValue struct {
	id   uint64
	refs []quadBatchPrimitiveValueRef
}

type quadBatchPrimitiveValueRef struct {
	match int
	dir   quad.Direction
}

var (
	_ QuadFilterBatchCollector = (*QuadStore)(nil)
	_ graph.QuadStore          = (*QuadStore)(nil)
)
