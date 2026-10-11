// Copyright 2017 The Cayley Authors. All rights reserved.
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
	"encoding/binary"
	"slices"
	"sync"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/internal/lru"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/pquads"
	"github.com/aperturerobotics/cayley/query/shape"
	"github.com/pkg/errors"
)

// QuadStore stores graph primitives and quad postings in a hierarchical KV database.
type QuadStore struct {
	// db stores the graph's primitive log, nodes, counters, and postings.
	db kv.KV
	// indexMeta records a newly selected layout with each mutation transaction.
	// It is immutable after opening and nil when stored metadata already matches.
	indexMeta []byte

	// indexes guards the active layout and cached duplicate-lookup indexes.
	indexes struct {
		// RWMutex guards all and exists.
		sync.RWMutex
		// all is the complete active posting layout.
		all []QuadIndex
		// exists caches the indexes used for exact quad lookup.
		exists []QuadIndex
	}

	// valueLRU caches resolved IRI node IDs.
	valueLRU *lru.Cache
	// quadExists caches exact quad membership under writer.
	quadExists map[refs.QuadHash]struct{}

	// writer serializes durable graph updates and their pending postings.
	writer sync.Mutex
	// mapBucket holds additions pending the current writer transaction's flush.
	mapBucket map[string]map[string]*indexPosting
}

// newQuadStore binds a database before selecting its index layout.
func newQuadStore(db kv.KV) *QuadStore {
	return &QuadStore{db: db}
}

// Init requires no initial records; graph metadata is created by writes or reindexing.
func Init(context.Context, kv.KV, graph.Options) error {
	return nil
}

const (
	// OptAssumeDefaultIdx skips loading index metadata when the caller guarantees
	// that the stored graph uses DefaultQuadIndexes.
	OptAssumeDefaultIdx = "assume_default_idx"
	// OptQuadIndexes requests a []QuadIndex layout, checked against stored metadata.
	// A different nonempty layout requires offline reindexing before opening.
	// Empty stores take the requested layout. It overrides OptAssumeDefaultIdx.
	OptQuadIndexes = "quad_indexes"
)

// New opens a graph with stored, default, or explicitly supplied quad indexes.
func New(ctx context.Context, kv kv.KV, opt graph.Options) (graph.QuadStore, error) {
	// Refuse graph access until an interrupted rebuild has completed.
	qs := newQuadStore(kv)
	if err := qs.checkIndexRebuild(ctx, opt); err != nil {
		return nil, err
	}

	// Select indexes only after checking the stored layout.
	assumeDefaultIdx, _ := opt.BoolKey(OptAssumeDefaultIdx, false)
	switch {
	case opt[OptQuadIndexes] != nil:
		indexes, ok := opt[OptQuadIndexes].([]QuadIndex)
		if !ok {
			return nil, errors.New("quad_indexes requires a []QuadIndex")
		}
		indexes, err := cloneQuadIndexes(indexes)
		if err != nil {
			return nil, err
		}
		if err := qs.selectQuadIndexes(ctx, indexes); err != nil {
			return nil, err
		}
	case assumeDefaultIdx:
		qs.indexes.all = DefaultQuadIndexes
	default:
		list, err := qs.readIndexesMeta(ctx)
		if err != nil {
			return nil, err
		}
		qs.indexes.all = list
	}

	// Keep node and exact-quad caches local to this graph handle.
	qs.valueLRU = lru.New(2000)
	qs.quadExists = make(map[refs.QuadHash]struct{})
	return qs, nil
}

// flushMetaCache records the selected layout and changed counters with the graph mutation.
func (qs *QuadStore) flushMetaCache(ctx context.Context, tx kv.Tx, cache *metaCache) error {
	// Keep first-write index metadata atomic with the primitives and postings it describes.
	if len(qs.indexMeta) != 0 {
		if err := tx.Put(ctx, kv.Key{keyMetaIndexes}, qs.indexMeta); err != nil {
			return err
		}
	}
	if cache == nil || len(cache.dirty) == 0 {
		return nil
	}

	// Flush changed counters in deterministic key order.
	keys := make([]string, 0, len(cache.dirty))
	for key := range cache.dirty {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if err := qs.putMetaInt(ctx, tx, key, cache.vals[key]); err != nil {
			return err
		}
	}
	clear(cache.dirty)
	return nil
}

// getMetaInt reads one signed metadata counter.
func (qs *QuadStore) getMetaInt(ctx context.Context, key string) (int64, error) {
	var v int64
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		// Require and decode the stored metadata value.
		val, err := tx.Get(ctx, metaBucket.AppendBytes([]byte(key)))
		if err == kv.ErrNotFound {
			return ErrNoBucket
		}
		if err != nil {
			return err
		}
		v, err = asInt64(val, 0)
		if err != nil {
			return err
		}
		return nil
	})
	return v, err
}

// getSize treats an unwritten graph as empty.
func (qs *QuadStore) getSize(ctx context.Context) (int64, error) {
	sz, err := qs.getMetaInt(ctx, "size")
	if err == ErrNoBucket {
		return 0, nil
	}
	return sz, err
}

// Size returns the number of live quad primitives.
func (qs *QuadStore) Size(ctx context.Context) (int64, error) {
	return qs.getSize(ctx)
}

// Stats reports stored quad counts and optionally counts live nodes.
func (qs *QuadStore) Stats(ctx context.Context, exact bool) (graph.Stats, error) {
	// Read the stored quad count before estimating the graph's node count.
	sz, err := qs.getMetaInt(ctx, "size")
	if err != nil {
		return graph.Stats{}, err
	}
	st := graph.Stats{
		Nodes: refs.Size{
			Value: sz / 3,
			Exact: false, // TODO(dennwc): store nodes count
		},
		Quads: refs.Size{
			Value: sz,
			Exact: true,
		},
	}

	// Count live node primitives when the caller requests exact statistics.
	if exact {
		st.Nodes.Value = 0
		it := qs.NodesAllIterator(ctx).Iterate(ctx)
		defer it.Close()
		for it.Next(ctx) {
			st.Nodes.Value++
		}
		if err := it.Err(); err != nil {
			return st, err
		}
		st.Nodes.Exact = true
	}
	return st, nil
}

// Close releases the backing database.
func (qs *QuadStore) Close() error {
	return qs.db.Close()
}

// asInt64 decodes a stored metadata counter or returns its empty default.
func asInt64(b []byte, empty int64) (int64, error) {
	if len(b) == 0 {
		return empty, nil
	}
	if len(b) != 8 {
		return 0, errors.Errorf("unexpected int size: %d", len(b))
	}
	v := int64(binary.LittleEndian.Uint64(b))
	return v, nil
}

// horizon returns the highest allocated primitive ID.
func (qs *QuadStore) horizon(ctx context.Context) int64 {
	h, _ := qs.getMetaInt(ctx, "horizon")
	return h
}

// ValuesOf resolves graph references to node values in caller order.
func (qs *QuadStore) ValuesOf(ctx context.Context, vals []graph.Ref) ([]quad.Value, error) {
	// Resolve prefetched values and collect durable node IDs for one read.
	out := make([]quad.Value, len(vals))
	var (
		inds  []int
		irefs []uint64
	)
	for i, v := range vals {
		if v == nil {
			continue
		}
		if pv, ok := v.(refs.PreFetchedValue); ok {
			out[i] = pv.NameOf()
			continue
		}
		switch v := v.(type) {
		case Int64Value:
			if v == 0 {
				continue
			}
			inds = append(inds, i)
			irefs = append(irefs, uint64(v))
		default:
			return out, errors.Errorf("unknown type of graph.Ref; not meant for this quadstore. apparently a %#v", v)
		}
	}
	if len(irefs) == 0 {
		return out, nil
	}

	// Read and decode the unresolved node values in caller order.
	prim, err := qs.getPrimitives(ctx, irefs)
	if err != nil {
		return out, err
	}
	var last error
	for i, p := range prim {
		if p == nil || !p.IsNode() {
			continue
		}
		qv, err := pquads.UnmarshalValue(ctx, p.Value)
		if err != nil {
			last = err
			continue
		}
		out[inds[i]] = qv
	}
	return out, last
}

// RefsOf resolves node values to durable graph references in caller order.
func (qs *QuadStore) RefsOf(ctx context.Context, nodes []quad.Value) ([]graph.Ref, error) {
	values := make([]graph.Ref, len(nodes))
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		for i, node := range nodes {
			value, err := qs.resolveQuadValue(ctx, tx, node)
			if err != nil {
				return err
			}
			values[i] = Int64Value(value)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

// NameOf resolves one graph reference to its node value.
func (qs *QuadStore) NameOf(ctx context.Context, v graph.Ref) (quad.Value, error) {
	vals, err := qs.ValuesOf(ctx, []graph.Ref{v})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting NameOf %d", v)
	}
	return vals[0], nil
}

// Quad resolves a quad primitive to its directional node values.
func (qs *QuadStore) Quad(ctx context.Context, k graph.Ref) (quad.Quad, error) {
	// Require a quad primitive before resolving its directional node values.
	key, ok := k.(*proto.Primitive)
	if !ok {
		return quad.Quad{}, errors.Errorf("passed value was not a quad primitive: %T", k)
	}

	// Resolve every direction under one database read transaction.
	var v quad.Quad
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		var err error
		v, err = qs.primitiveToQuad(ctx, tx, key)
		return err
	})
	if err == kv.ErrNotFound {
		err = nil
	}
	if err != nil {
		err = errors.Wrapf(err, "error fetching quad %#v", key)
	}
	return v, err
}

// primitiveToQuad resolves each direction under the caller's transaction.
func (qs *QuadStore) primitiveToQuad(ctx context.Context, tx kv.Tx, p *proto.Primitive) (quad.Quad, error) {
	q := &quad.Quad{}
	for _, dir := range quad.Directions {
		v := p.GetDirection(dir)
		val, err := qs.getValFromLog(ctx, tx, v)
		if err != nil {
			return *q, err
		}
		q.Set(dir, val)
	}
	return *q, nil
}

// getValFromLog resolves a live node primitive, returning nil for an absent node.
func (qs *QuadStore) getValFromLog(ctx context.Context, tx kv.Tx, k uint64) (quad.Value, error) {
	// Resolve a live node value from the primitive log.
	if k == 0 {
		return nil, nil
	}
	p, err := qs.getPrimitiveFromLog(ctx, tx, k)
	if err == kv.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if p.Deleted {
		return nil, nil
	}
	return pquads.UnmarshalValue(ctx, p.Value)
}

// ValueOf resolves a node value, returning nil when it is absent.
func (qs *QuadStore) ValueOf(ctx context.Context, s quad.Value) (graph.Ref, error) {
	// Resolve the node value under one database read transaction.
	var out Int64Value
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		v, err := qs.resolveQuadValue(ctx, tx, s)
		out = Int64Value(v)
		return err
	})
	if err != nil {
		return nil, err
	}
	if out == 0 {
		return nil, nil
	}
	return out, nil
}

// QuadDirection returns a quad primitive's node reference for one direction.
func (qs *QuadStore) QuadDirection(ctx context.Context, val graph.Ref, d quad.Direction) (graph.Ref, error) {
	p, ok := val.(*proto.Primitive)
	if !ok {
		return nil, nil
	}
	switch d {
	case quad.Subject:
		return Int64Value(p.Subject), nil
	case quad.Predicate:
		return Int64Value(p.Predicate), nil
	case quad.Object:
		return Int64Value(p.Object), nil
	case quad.Label:
		if p.Label == 0 {
			return nil, nil
		}
		return Int64Value(p.Label), nil
	}
	return nil, nil
}

// getPrimitives reads primitive records under one database transaction.
func (qs *QuadStore) getPrimitives(ctx context.Context, vals []uint64) ([]*proto.Primitive, error) {
	tx, err := qs.db.Tx(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	return qs.getPrimitivesFromLog(ctx, tx, vals)
}

// _ is a type assertion.
var (
	_ refs.BatchNamer = (*QuadStore)(nil)
	_ shape.Optimizer = (*QuadStore)(nil)
)
