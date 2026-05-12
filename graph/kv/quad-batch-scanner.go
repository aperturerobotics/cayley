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
		dirs := make([]quad.Direction, 0, len(out[filterIdx].all))
		for dir := range out[filterIdx].all {
			dirs = append(dirs, dir)
		}
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

func (qs *QuadStore) resolveQuadValuesForBatch(ctx context.Context, tx kv.Tx, refs []quadBatchValueRef) ([]uint64, error) {
	vals := make([]quad.Value, len(refs))
	for i, ref := range refs {
		vals[i] = ref.value
	}
	return qs.resolveQuadValues(ctx, tx, vals)
}

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
	for it.Next(ctx) {
		ids, err := decodeIndex(it.Val())
		if err != nil {
			return err
		}
		for len(ids) > 0 {
			batch := ids
			if len(batch) > nextBatch {
				batch = batch[:nextBatch]
			}
			prims, err := qs.getPrimitivesFromLog(ctx, tx, batch)
			if err != nil {
				return err
			}
			for _, prim := range prims {
				if prim == nil || prim.Deleted {
					continue
				}
				for _, filterIdx := range group.filterIndexes {
					if filled[filterIdx] || !primitiveMatchesQuadFilter(prim, filters[filterIdx]) {
						continue
					}
					q, err := qs.primitiveToQuadBatch(ctx, tx, prim)
					if err != nil {
						return err
					}
					results[filterIdx] = append(results[filterIdx], q)
					if limit != 0 && uint32(len(results[filterIdx])) >= limit {
						filled[filterIdx] = true
						remaining--
					}
				}
				if limit != 0 && remaining == 0 {
					return nil
				}
			}
			ids = ids[len(batch):]
		}
	}
	return it.Err()
}

func (qs *QuadStore) collectAllQuadsForBatchFilter(ctx context.Context, tx kv.Tx, filter resolvedQuadBatchFilter, results *[]quad.Quad, limit uint32) error {
	horizon, err := qs.getMetaIntTx(ctx, tx, "horizon")
	if err == kv.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}
	var id uint64
	for id < uint64(horizon) {
		ids := make([]uint64, 0, nextBatch)
		for range nextBatch {
			id++
			if id > uint64(horizon) {
				break
			}
			ids = append(ids, id)
		}
		prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
		if err != nil {
			return err
		}
		for _, prim := range prims {
			if prim == nil || prim.Deleted || prim.IsNode() || !primitiveMatchesQuadFilter(prim, filter) {
				continue
			}
			q, err := qs.primitiveToQuadBatch(ctx, tx, prim)
			if err != nil {
				return err
			}
			*results = append(*results, q)
			if limit != 0 && uint32(len(*results)) >= limit {
				return nil
			}
		}
	}
	return nil
}

func primitiveMatchesQuadFilter(prim *proto.Primitive, filter resolvedQuadBatchFilter) bool {
	for dir, id := range filter.all {
		if prim.GetDirection(dir) != id {
			return false
		}
	}
	return true
}

func (qs *QuadStore) primitiveToQuadBatch(ctx context.Context, tx kv.Tx, prim *proto.Primitive) (quad.Quad, error) {
	var ids []uint64
	var dirs []quad.Direction
	for _, dir := range quad.Directions {
		id := prim.GetDirection(dir)
		if id == 0 {
			continue
		}
		ids = append(ids, id)
		dirs = append(dirs, dir)
	}
	prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
	if err != nil {
		return quad.Quad{}, err
	}
	var out quad.Quad
	for i, p := range prims {
		if p == nil {
			continue
		}
		value, err := pquads.UnmarshalValue(ctx, p.Value)
		if err != nil {
			return out, err
		}
		out.Set(dirs[i], value)
	}
	return out, nil
}

var (
	_ QuadFilterBatchCollector = (*QuadStore)(nil)
	_ graph.QuadStore          = (*QuadStore)(nil)
)
