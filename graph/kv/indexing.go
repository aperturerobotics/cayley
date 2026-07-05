// Copyright 2016 The Cayley Authors. All rights reserved.
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
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"runtime/trace"
	"slices"
	"sort"
	"strconv"

	"github.com/aperturerobotics/cayley/clog"
	"github.com/aperturerobotics/cayley/graph"
	graphlog "github.com/aperturerobotics/cayley/graph/log"
	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/pquads"
	b58 "github.com/mr-tron/base58/base58"

	"github.com/aperturerobotics/cayley/kv"
)

var (
	metaBucket = kv.Key{[]byte("m")}
	logIndex   = kv.Key{[]byte("l")}

	keyMetaIndexes = []byte("i")

	DefaultQuadIndexes = []QuadIndex{
		// First index optimizes forward traversals. Getting all relations for a node should
		// also be reasonably fast (prefix scan).
		{Dirs: []quad.Direction{quad.Subject, quad.Predicate}},

		// Second index helps with reverse traversals as well as full quad lookups.
		// It also prevents issues with super-nodes, since most of those are values
		// with a high in-degree.
		{Dirs: []quad.Direction{quad.Object, quad.Predicate, quad.Subject}},
	}
)

type QuadIndex struct {
	Dirs   []quad.Direction
	Unique bool
}

// CompareQuadDirections compares two slices of quad directions for equality.
func CompareQuadDirections(a, b []quad.Direction) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CompareQuadIndexes compares two slices of quad indexes for equality.
func CompareQuadIndexes(a, b []QuadIndex) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

func (ind QuadIndex) Equal(ot QuadIndex) bool {
	if ind.Unique != ot.Unique {
		return false
	}
	if !CompareQuadDirections(ind.Dirs, ot.Dirs) {
		return false
	}
	return true
}

func (ind QuadIndex) Key(vals []uint64) kv.Key {
	var buf bytes.Buffer
	buf.Grow(12 * len(vals))
	for i := range vals {
		if i != 0 {
			_, _ = buf.WriteRune(rune(':'))
		}
		_, _ = buf.Write(uint64KeyBytes(vals[i]))
	}
	// Append delimiter for partial keys so prefix scans don't bleed
	// across ID boundaries (e.g. "3" matching "32").
	if len(vals) > 0 && len(vals) < len(ind.Dirs) {
		_, _ = buf.WriteRune(rune(':'))
	}
	return ind.bucket().AppendBytes(buf.Bytes())
}

func (ind QuadIndex) KeyFor(p *proto.Primitive) kv.Key {
	vals := make([]uint64, len(ind.Dirs))
	for i, d := range ind.Dirs {
		vals[i] = p.GetDirection(d)
	}
	return ind.Key(vals)
}

func (ind QuadIndex) bucket() kv.Key {
	buf := make([]byte, len(ind.Dirs))
	for i, d := range ind.Dirs {
		buf[i] = d.Prefix() // a s p c o
		// prevent nil character
		if buf[i] == '\x00' {
			buf[i] = '0'
		}
	}
	key := make(kv.Key, 1, 2)
	key[0] = buf
	return key
}

func bucketForVal() kv.Key {
	return kv.Key{[]byte{'v'}}
}

func bucketForValRefs() kv.Key {
	return kv.Key{[]byte{'n'}}
}

type metaCache struct {
	vals   map[string]int64
	loaded map[string]struct{}
	dirty  map[string]struct{}
}

func newMetaCache() *metaCache {
	return &metaCache{
		vals:   make(map[string]int64),
		loaded: make(map[string]struct{}),
		dirty:  make(map[string]struct{}),
	}
}

// writeIndexesMeta writes metadata about current indexes to the KV database,
// so we can read this information back later.
func (qs *QuadStore) writeIndexesMeta(ctx context.Context) error {
	data, err := encodeQuadIndexes(qs.indexes.all)
	if err != nil {
		return err
	}
	return kv.Update(ctx, qs.db, func(tx kv.Tx) error {
		return tx.Put(ctx, kv.Key{keyMetaIndexes}, data)
	})
}

// readIndexesMeta read metadata about current indexes from the KV database.
// If no indexes are set, it returns a list of legacy indexes to preserve backward compatibility.
func (qs *QuadStore) readIndexesMeta(ctx context.Context) ([]QuadIndex, error) {
	tx, err := qs.db.Tx(ctx, false)
	if err != nil {
		return nil, err
	}
	defer tx.Close()
	val, err := tx.Get(ctx, kv.Key{keyMetaIndexes})
	if err == kv.ErrNotFound {
		return DefaultQuadIndexes, nil
	} else if err != nil {
		return nil, err
	}
	out, err := decodeQuadIndexes(val)
	if err != nil {
		return nil, fmt.Errorf("cannot decode indexes: %v", err)
	} else if len(out) == 0 {
		return DefaultQuadIndexes, nil
	}
	return out, nil
}

func encodeQuadIndexes(indexes []QuadIndex) ([]byte, error) {
	list := &QuadIndexList{
		Indexes: make([]*QuadIndexMeta, len(indexes)),
	}
	for i, index := range indexes {
		meta := &QuadIndexMeta{
			Dirs:   make([]uint32, len(index.Dirs)),
			Unique: index.Unique,
		}
		for j, dir := range index.Dirs {
			if dir == quad.Any || dir.Prefix() == 0 {
				return nil, errors.New("invalid direction")
			}
			meta.Dirs[j] = uint32(dir)
		}
		list.Indexes[i] = meta
	}
	return list.MarshalVT()
}

func decodeQuadIndexes(data []byte) ([]QuadIndex, error) {
	list := &QuadIndexList{}
	if err := list.UnmarshalVT(data); err != nil {
		return nil, err
	}
	indexes := make([]QuadIndex, len(list.GetIndexes()))
	for i, meta := range list.GetIndexes() {
		indexes[i].Unique = meta.GetUnique()
		indexes[i].Dirs = make([]quad.Direction, len(meta.GetDirs()))
		for j, raw := range meta.GetDirs() {
			dir := quad.Direction(raw)
			if dir == quad.Any || dir.Prefix() == 0 {
				return nil, errors.New("invalid direction")
			}
			indexes[i].Dirs[j] = dir
		}
	}
	return indexes, nil
}

func (qs *QuadStore) resolveValDeltas(ctx context.Context, tx kv.Tx, deltas []graphlog.NodeUpdate, fnc func(i int, id uint64)) error {
	inds := make([]int, 0, len(deltas))
	keys := make([]kv.Key, 0, len(deltas))
	for i, d := range deltas {
		if iri, ok := d.Val.(quad.IRI); ok {
			if x, ok := qs.valueLRU.Get(string(iri)); ok {
				fnc(i, x.(uint64))
				continue
			}
		} else if d.Val == nil {
			fnc(i, 0)
			continue
		}
		inds = append(inds, i)
		keys = append(keys, bucketKeyForHash(d.Hash[:]))
	}
	if len(keys) == 0 {
		return nil
	}
	resp, err := tx.GetBatch(ctx, keys)
	if err != nil {
		return err
	}
	for i, b := range resp {
		if len(b) == 0 {
			fnc(inds[i], 0)
			continue
		}
		ind := inds[i]
		id, _ := binary.Uvarint(b)
		d := &deltas[ind]
		if iri, ok := d.Val.(quad.IRI); ok && id != 0 {
			qs.valueLRU.Put(string(iri), id)
		}
		fnc(ind, id)
	}
	return nil
}

func (qs *QuadStore) getMetaIntTx(ctx context.Context, tx kv.Tx, key string) (int64, error) {
	val, err := tx.Get(ctx, metaBucket.AppendBytes([]byte(key)))
	if err == kv.ErrNotFound {
		return 0, err
	} else if err != nil {
		return 0, fmt.Errorf("cannot get horizon value: %v", err)
	}
	return int64(binary.LittleEndian.Uint64(val)), nil
}

func (qs *QuadStore) incMetaInt(
	ctx context.Context,
	tx kv.Tx,
	cache *metaCache,
	key string,
	n int64,
) (int64, error) {
	if n == 0 {
		return 0, nil
	}
	if cache != nil {
		if _, ok := cache.loaded[key]; !ok {
			getCtx, getTask := trace.NewTask(ctx, "cayley/kv/inc-meta-int/get-meta")
			v, err := qs.getMetaIntTx(getCtx, tx, key)
			getTask.End()
			if err != nil && err != kv.ErrNotFound {
				return 0, fmt.Errorf("cannot get %s: %v", key, err)
			}
			cache.vals[key] = v
			cache.loaded[key] = struct{}{}
		}
		start := cache.vals[key]
		cache.vals[key] = start + n
		cache.dirty[key] = struct{}{}
		return start, nil
	}
	getCtx, getTask := trace.NewTask(ctx, "cayley/kv/inc-meta-int/get-meta")
	v, err := qs.getMetaIntTx(getCtx, tx, key)
	getTask.End()
	if err != nil && err != kv.ErrNotFound {
		return 0, fmt.Errorf("cannot get %s: %v", key, err)
	}
	start := v
	v += n
	if err := qs.putMetaInt(ctx, tx, key, v); err != nil {
		return 0, err
	}
	return start, nil
}

func (qs *QuadStore) putMetaInt(ctx context.Context, tx kv.Tx, key string, v int64) error {
	buf := make([]byte, 8) // bolt needs all slices available on Commit
	binary.LittleEndian.PutUint64(buf, uint64(v))

	putCtx, putTask := trace.NewTask(ctx, "cayley/kv/inc-meta-int/put-meta")
	err := tx.Put(putCtx, metaBucket.AppendBytes([]byte(key)), buf)
	putTask.End()
	if err != nil {
		return fmt.Errorf("cannot inc %s: %v", key, err)
	}
	return nil
}

func (qs *QuadStore) flushMetaCache(ctx context.Context, tx kv.Tx, cache *metaCache) error {
	if cache == nil || len(cache.dirty) == 0 {
		return nil
	}
	keys := make([]string, 0, len(cache.dirty))
	for key := range cache.dirty {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := qs.putMetaInt(ctx, tx, key, cache.vals[key]); err != nil {
			return err
		}
	}
	clear(cache.dirty)
	return nil
}

func (qs *QuadStore) genIDs(ctx context.Context, tx kv.Tx, cache *metaCache, n int) (uint64, error) {
	if n == 0 {
		return 0, nil
	}
	start, err := qs.incMetaInt(ctx, tx, cache, "horizon", int64(n))
	if err != nil {
		return 0, err
	}
	return uint64(start + 1), nil
}

type nodeUpdate struct {
	Ind int
	ID  uint64
	graphlog.NodeUpdate
}

func (qs *QuadStore) incNodesCnt(ctx context.Context, tx kv.Tx, deltas, newDeltas []nodeUpdate) ([]int, error) {
	var buf [binary.MaxVarintLen64]byte
	// increment nodes
	keys := make([]kv.Key, 0, len(deltas))
	for _, d := range deltas {
		keys = append(keys, bucketKeyForHashRefs(d.Hash))
	}
	getCtx, getTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/update-refcounts/get-refcounts")
	sizes, err := tx.GetBatch(getCtx, keys)
	getTask.End()
	if err != nil {
		return nil, err
	}
	var del []int
	_, updTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/update-refcounts/update-existing")
	for i, d := range deltas {
		k := keys[i]
		var sz int64
		if sizes[i] != nil {
			szu, _ := binary.Uvarint(sizes[i])
			sz = int64(szu)
			sizes[i] = nil // cannot reuse buffer since it belongs to kv
		}
		sz += int64(d.RefInc)
		if sz <= 0 {
			if err := tx.Del(ctx, k); err != nil {
				return del, err
			}
			del = append(del, i)
			continue
		}
		n := binary.PutUvarint(buf[:], uint64(sz))
		val := append([]byte{}, buf[:n]...)
		if err := tx.Put(ctx, k, val); err != nil {
			updTask.End()
			return del, err
		}
	}
	updTask.End()
	// create new nodes
	newCtx, newTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/update-refcounts/create-new")
	for _, d := range newDeltas {
		n := binary.PutUvarint(buf[:], uint64(d.RefInc))
		val := append([]byte{}, buf[:n]...)
		if err := tx.Put(newCtx, bucketKeyForHashRefs(d.Hash), val); err != nil {
			newTask.End()
			return nil, err
		}
	}
	newTask.End()
	return del, nil
}

type resolvedNode struct {
	ID  uint64
	New bool
}

func (qs *QuadStore) incNodes(
	ctx context.Context,
	tx kv.Tx,
	cache *metaCache,
	deltas []graphlog.NodeUpdate,
	resolved map[refs.ValueHash]uint64,
) (map[refs.ValueHash]resolvedNode, error) {
	var (
		ins            []nodeUpdate
		unresolved     = make([]graphlog.NodeUpdate, 0, len(deltas))
		unresolvedInds = make([]int, 0, len(deltas))
		upd            = make([]nodeUpdate, 0, len(deltas))
		ids            = make(map[refs.ValueHash]resolvedNode, len(deltas))
	)
	handleResolved := func(i int, id uint64) {
		if id == 0 {
			ins = append(ins, nodeUpdate{Ind: i, NodeUpdate: deltas[i]})
			return
		}
		ids[deltas[i].Hash] = resolvedNode{ID: id}
		if deltas[i].RefInc != 0 {
			upd = append(upd, nodeUpdate{Ind: i, ID: id, NodeUpdate: deltas[i]})
		}
	}
	for i, d := range deltas {
		if resolved != nil {
			if id, ok := resolved[d.Hash]; ok {
				handleResolved(i, id)
				continue
			}
		}
		unresolved = append(unresolved, d)
		unresolvedInds = append(unresolvedInds, i)
	}
	if len(unresolved) != 0 {
		resolveCtx, resolveTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/resolve-values")
		err := qs.resolveValDeltas(resolveCtx, tx, unresolved, func(i int, id uint64) {
			handleResolved(unresolvedInds[i], id)
		})
		resolveTask.End()
		if err != nil {
			return ids, err
		}
	}
	if len(ins) != 0 {
		// preallocate IDs
		idCtx, idTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/gen-node-ids")
		start, err := qs.genIDs(idCtx, tx, cache, len(ins))
		idTask.End()
		if err != nil {
			return ids, err
		}
		// create and index new nodes
		_, indexTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/index-new-nodes")
		for i, iv := range ins {
			id := start + uint64(i)
			primCtx, primTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/index-new-nodes/build-node-primitive")
			_ = primCtx
			node, err := createNodePrimitive(iv.Val)
			primTask.End()
			if err != nil {
				indexTask.End()
				return ids, err
			}

			node.ID = id
			ids[iv.Hash] = resolvedNode{ID: id, New: true}
			if err := qs.indexNode(ctx, tx, node, iv.Val); err != nil {
				indexTask.End()
				return ids, err
			}
			ins[i].ID = id
		}
		indexTask.End()
	}
	countCtx, countTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/update-refcounts")
	_, err := qs.incNodesCnt(countCtx, tx, upd, ins)
	countTask.End()
	return ids, err
}

func (qs *QuadStore) decNodes(ctx context.Context, tx kv.Tx, deltas []graphlog.NodeUpdate, nodes map[refs.ValueHash]uint64) error {
	upds := make([]nodeUpdate, 0, len(deltas))
	for i, d := range deltas {
		id := nodes[d.Hash]
		if id == 0 || d.RefInc == 0 {
			continue
		}
		upds = append(upds, nodeUpdate{Ind: i, ID: id, NodeUpdate: d})
	}
	del, err := qs.incNodesCnt(ctx, tx, upds, nil)
	if err != nil {
		return err
	}
	for _, i := range del {
		d := upds[i]
		key := bucketKeyForHash(d.Hash[:])
		if err = tx.Del(ctx, key); err != nil {
			return err
		}
		if iri, ok := d.Val.(quad.IRI); ok {
			qs.valueLRU.Del(string(iri))
		}
		node, err := createNodePrimitive(d.Val)
		if err != nil {
			return err
		}
		node.ID = d.ID
		node.Deleted = true
		if err := qs.addToLog(ctx, tx, node); err != nil {
			return err
		}
	}
	return nil
}

func (qs *QuadStore) NewQuadWriter(ctx context.Context) (quad.WriteCloser, error) {
	return &quadWriter{qs: qs}, nil
}

type quadWriter struct {
	qs  *QuadStore
	tx  kv.Tx
	mc  *metaCache
	err error
	n   int
}

func (w *quadWriter) WriteQuad(ctx context.Context, q quad.Quad) error {
	_, err := w.WriteQuads(ctx, []quad.Quad{q})
	return err
}

func (w *quadWriter) flush(ctx context.Context) error {
	w.n = 0
	if err := w.qs.flushMapBucket(ctx, w.tx); err != nil {
		w.err = err
		return err
	}
	if err := w.qs.flushMetaCache(ctx, w.tx, w.mc); err != nil {
		w.err = err
		return err
	}
	if err := w.tx.Commit(ctx); err != nil {
		w.qs.writer.Unlock()
		w.tx = nil
		w.mc = nil
		w.err = err
		return err
	}
	tx, err := w.qs.db.Tx(ctx, true)
	if err != nil {
		w.qs.writer.Unlock()
		w.err = err
		return err
	}
	w.tx = tx
	w.mc = newMetaCache()
	return nil
}

func (w *quadWriter) WriteQuads(ctx context.Context, buf []quad.Quad) (int, error) {
	if w.tx == nil {
		w.qs.writer.Lock()
		tx, err := w.qs.db.Tx(ctx, true)
		if err != nil {
			w.qs.writer.Unlock()
			w.err = err
			return 0, err
		}
		w.tx = tx
		w.mc = newMetaCache()
	}
	deltas := graphlog.InsertQuads(buf)
	if _, err := w.qs.applyAddDeltas(w.tx, w.mc, nil, deltas, graph.IgnoreOpts{IgnoreDup: true}, nil); err != nil {
		w.err = err
		return 0, err
	}
	w.n += len(buf)
	if w.n >= quad.DefaultBatch*20 {
		if err := w.flush(ctx); err != nil {
			return 0, err
		}
	}
	return len(buf), nil
}

func (w *quadWriter) Close() error {
	if w.tx == nil {
		return w.err
	}
	defer w.qs.writer.Unlock()

	if w.err != nil {
		_ = w.tx.Close()
		w.tx = nil
		return w.err
	}

	ctx := context.TODO()
	// flush quad indexes and commit
	err := w.qs.flushMapBucket(ctx, w.tx)
	if err != nil {
		_ = w.tx.Close()
		w.tx = nil
		w.mc = nil
		return err
	}
	err = w.qs.flushMetaCache(ctx, w.tx, w.mc)
	if err != nil {
		_ = w.tx.Close()
		w.tx = nil
		w.mc = nil
		return err
	}
	err = w.tx.Commit(ctx)
	w.tx = nil
	w.mc = nil
	return err
}

func (qs *QuadStore) precheckAddDeltas(
	ctx context.Context,
	tx kv.Tx,
	in []graph.Delta,
	ignoreOpts graph.IgnoreOpts,
) ([]graph.Delta, map[refs.ValueHash]uint64, error) {
	if len(in) == 0 {
		return in, nil, nil
	}
	var (
		qhashes     []refs.QuadHash
		check       []bool
		nodesByHash map[refs.ValueHash]graphlog.NodeUpdate
	)
	for i, d := range in {
		if d.Action != graph.Add {
			continue
		}
		qh := quadHashOf(d.Quad)
		if ignoreOpts.IgnoreDup {
			if _, ok := qs.quadExists[qh]; !ok {
				continue
			}
		}
		if qhashes == nil {
			qhashes = make([]refs.QuadHash, len(in))
			check = make([]bool, len(in))
			nodesByHash = make(map[refs.ValueHash]graphlog.NodeUpdate)
		}
		qhashes[i] = qh
		check[i] = true
		for _, dir := range quad.Directions {
			v := d.Quad.Get(dir)
			if v == nil {
				continue
			}
			h := qh.Get(dir)
			if _, ok := nodesByHash[h]; !ok {
				nodesByHash[h] = graphlog.NodeUpdate{Hash: h, Val: v}
			}
		}
	}
	if len(nodesByHash) == 0 {
		return in, nil, nil
	}

	nodesToResolve := make([]graphlog.NodeUpdate, 0, len(nodesByHash))
	for _, n := range nodesByHash {
		nodesToResolve = append(nodesToResolve, n)
	}
	slices.SortFunc(nodesToResolve, func(a, b graphlog.NodeUpdate) int {
		return bytes.Compare(a.Hash[:], b.Hash[:])
	})

	nodes := make(map[refs.ValueHash]uint64, len(nodesToResolve))
	resolveCtx, resolveTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/precheck-adds/resolve-nodes")
	err := qs.resolveValDeltas(resolveCtx, tx, nodesToResolve, func(i int, id uint64) {
		nodes[nodesToResolve[i].Hash] = id
	})
	resolveTask.End()
	if err != nil {
		return nil, nil, err
	}

	kept := make([]graph.Delta, 0, len(in))
	seen := make(map[refs.QuadHash]struct{}, len(in))
	for i, d := range in {
		if d.Action != graph.Add {
			kept = append(kept, d)
			continue
		}
		if !check[i] {
			kept = append(kept, d)
			continue
		}
		qh := qhashes[i]
		if _, ok := seen[qh]; ok {
			continue
		}
		seen[qh] = struct{}{}

		link := &proto.Primitive{}
		canExist := true
		for _, dir := range quad.Directions {
			h := qh.Get(dir)
			if !h.Valid() {
				continue
			}
			id := nodes[h]
			if id == 0 {
				canExist = false
				break
			}
			link.SetDirection(dir, id)
		}
		if canExist {
			checkCtx, checkTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/precheck-adds/check-link-exists")
			p, err := qs.hasPrimitive(checkCtx, tx, link, false)
			checkTask.End()
			if err != nil {
				return nil, nil, err
			}
			if p != nil {
				if ignoreOpts.IgnoreDup {
					continue
				}
				return nil, nil, &graph.DeltaError{Delta: d, Err: graph.ErrQuadExists}
			}
		}
		kept = append(kept, d)
	}
	return kept, nodes, nil
}

func (qs *QuadStore) filterDuplicateAddDeltas(
	ctx context.Context,
	tx kv.Tx,
	in []graph.Delta,
	deltas *graphlog.Deltas,
	ignoreOpts graph.IgnoreOpts,
) (map[refs.ValueHash]uint64, error) {
	if len(deltas.QuadAdd) == 0 {
		return nil, nil
	}

	nodes := make(map[refs.ValueHash]uint64, len(deltas.IncNode))
	resolveCtx, resolveTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/resolve-duplicate-check-nodes")
	err := qs.resolveValDeltas(resolveCtx, tx, deltas.IncNode, func(i int, id uint64) {
		nodes[deltas.IncNode[i].Hash] = id
	})
	resolveTask.End()
	if err != nil {
		return nil, err
	}

	incIndexes := make(map[refs.ValueHash]int, len(deltas.IncNode))
	for i, d := range deltas.IncNode {
		incIndexes[d.Hash] = i
	}
	decrementRefs := func(q refs.QuadHash) {
		for _, h := range q.Dirs() {
			if !h.Valid() {
				continue
			}
			deltas.IncNode[incIndexes[h]].RefInc--
		}
	}

	kept := deltas.QuadAdd[:0]
	seen := make(map[refs.QuadHash]struct{}, len(deltas.QuadAdd))
	for _, q := range deltas.QuadAdd {
		if _, ok := seen[q.Quad]; ok {
			decrementRefs(q.Quad)
			continue
		}
		seen[q.Quad] = struct{}{}

		link := &proto.Primitive{}
		canExist := true
		for _, dir := range quad.Directions {
			h := q.Quad.Get(dir)
			if !h.Valid() {
				continue
			}
			id := nodes[h]
			if id == 0 {
				canExist = false
				break
			}
			link.SetDirection(dir, id)
		}
		if canExist {
			checkCtx, checkTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/check-link-exists")
			p, err := qs.hasPrimitive(checkCtx, tx, link, false)
			checkTask.End()
			if err != nil {
				return nil, err
			}
			if p != nil {
				if ignoreOpts.IgnoreDup {
					decrementRefs(q.Quad)
					continue
				}
				err = graph.ErrQuadExists
				if len(in) != 0 {
					return nil, &graph.DeltaError{Delta: in[q.Ind], Err: err}
				}
				return nil, err
			}
		}
		kept = append(kept, q)
	}
	deltas.QuadAdd = kept

	return nodes, nil
}

func (qs *QuadStore) applyAddDeltas(
	tx kv.Tx,
	cache *metaCache,
	in []graph.Delta,
	deltas *graphlog.Deltas,
	ignoreOpts graph.IgnoreOpts,
	resolved map[refs.ValueHash]uint64,
) (map[refs.ValueHash]resolvedNode, error) {
	ctx := context.TODO()

	if resolved == nil {
		filterCtx, filterTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/filter-duplicate-adds")
		var err error
		resolved, err = qs.filterDuplicateAddDeltas(filterCtx, tx, in, deltas, ignoreOpts)
		filterTask.End()
		if err != nil {
			return nil, err
		}
	}

	nodeCtx, nodeTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes")
	nodes, err := qs.incNodes(nodeCtx, tx, cache, deltas.IncNode, resolved)
	nodeTask.End()
	if err != nil {
		return nil, err
	}
	deltas.IncNode = nil

	links := make([]*proto.Primitive, 0, len(deltas.QuadAdd))
	linkHashes := make([]refs.QuadHash, 0, len(deltas.QuadAdd))
	qadd := make(map[[4]uint64]struct{}, len(deltas.QuadAdd))
	_, buildTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/build-links")
	for _, q := range deltas.QuadAdd {
		link := &proto.Primitive{}
		var qkey [4]uint64
		for i, dir := range quad.Directions {
			n, ok := nodes[q.Quad.Get(dir)]
			if !ok {
				continue
			}
			link.SetDirection(dir, n.ID)
			qkey[i] = n.ID
		}
		if _, ok := qadd[qkey]; ok {
			continue
		}
		qadd[qkey] = struct{}{}
		links = append(links, link)
		linkHashes = append(linkHashes, q.Quad)
	}
	buildTask.End()
	deltas.QuadAdd = nil

	idCtx, idTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/gen-link-ids")
	qstart, err := qs.genIDs(idCtx, tx, cache, len(links))
	idTask.End()
	if err != nil {
		return nil, err
	}
	for i := range links {
		links[i].ID = qstart + uint64(i)
	}
	newIDs := make(map[uint64]struct{}, len(nodes))
	for _, n := range nodes {
		if n.New {
			newIDs[n.ID] = struct{}{}
		}
	}
	indexCtx, indexTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/index-links")
	if err := qs.indexLinks(indexCtx, tx, cache, links, newIDs); err != nil {
		indexTask.End()
		return nil, err
	}
	indexTask.End()
	for _, qh := range linkHashes {
		qs.rememberQuadExists(qh)
	}
	return nodes, nil
}

func (qs *QuadStore) ApplyDeltas(ctx context.Context, in []graph.Delta, ignoreOpts graph.IgnoreOpts) error {
	ctx, task := trace.NewTask(ctx, "cayley/kv/apply-deltas")
	defer task.End()

	qs.writer.Lock()
	defer qs.writer.Unlock()
	txCtx, txTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/open-tx")
	tx, err := qs.db.Tx(txCtx, true)
	txTask.End()
	if err != nil {
		return err
	}
	defer tx.Close()

	precheckCtx, precheckTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/precheck-adds")
	var resolved map[refs.ValueHash]uint64
	in, resolved, err = qs.precheckAddDeltas(precheckCtx, tx, in, ignoreOpts)
	precheckTask.End()
	if err != nil {
		return err
	}

	_, splitTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/split-deltas")
	deltas := graphlog.SplitDeltas(in)
	splitTask.End()
	cache := newMetaCache()
	_, addTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas")
	nodes, err := qs.applyAddDeltas(tx, cache, in, deltas, ignoreOpts, resolved)
	addTask.End()
	if err != nil {
		return err
	}

	if len(deltas.QuadDel) != 0 || len(deltas.DecNode) != 0 {
		links := make([]*proto.Primitive, 0, len(deltas.QuadDel))
		// resolve all nodes that will be removed
		dnodes := make(map[refs.ValueHash]uint64, len(deltas.DecNode))
		resolveCtx, resolveTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/resolve-dec-nodes")
		if err := qs.resolveValDeltas(resolveCtx, tx, deltas.DecNode, func(i int, id uint64) {
			dnodes[deltas.DecNode[i].Hash] = id
		}); err != nil {
			resolveTask.End()
			return err
		}
		resolveTask.End()

		// check for existence and delete quads
		fixNodes := make(map[refs.ValueHash]int)
		_, checkTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/check-delete-quads")
		for _, q := range deltas.QuadDel {
			link := &proto.Primitive{}
			exists := true
			// resolve values of all quad directions
			// if any of the direction does not exists, the quad does not exists as well
			for _, dir := range quad.Directions {
				h := q.Quad.Get(dir)
				n, ok := nodes[h]
				if !ok {
					var id uint64
					id, ok = dnodes[h]
					n.ID = id
				}
				if !ok {
					exists = exists && !h.Valid()
					continue
				}
				link.SetDirection(dir, n.ID)
			}
			if exists {
				p, err := qs.hasPrimitive(ctx, tx, link, true)
				if err != nil {
					return err
				} else if p == nil || p.Deleted {
					exists = false
				} else {
					link = p.CloneVT()
				}
			}
			if !exists {
				if !ignoreOpts.IgnoreMissing {
					return &graph.DeltaError{Delta: in[q.Ind], Err: graph.ErrQuadNotExist}
				}
				// revert counters for all directions of this quad
				for _, dir := range quad.Directions {
					if h := q.Quad.Get(dir); h.Valid() {
						fixNodes[h]++
					}
				}
				continue
			}
			links = append(links, link)
		}
		checkTask.End()
		deltas.QuadDel = nil
		markCtx, markTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/mark-links-dead")
		if err := qs.markLinksDead(markCtx, tx, cache, links); err != nil {
			markTask.End()
			return err
		}
		markTask.End()

		// we decremented some nodes that has non-existent quads - let's fix this
		if len(fixNodes) != 0 {
			for i, n := range deltas.DecNode {
				if dn := fixNodes[n.Hash]; dn != 0 {
					deltas.DecNode[i].RefInc += dn
				}
			}
		}

		// finally decrement and remove nodes
		decCtx, decTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/dec-nodes")
		if err := qs.decNodes(decCtx, tx, deltas.DecNode, dnodes); err != nil {
			decTask.End()
			return err
		}
		decTask.End()
		deltas = nil
		dnodes = nil
	}
	// flush quad indexes and commit
	flushCtx, flushTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/flush-map-bucket")
	err = qs.flushMapBucket(flushCtx, tx)
	flushTask.End()
	if err != nil {
		return err
	}
	if err := qs.flushMetaCache(ctx, tx, cache); err != nil {
		return err
	}
	commitCtx, commitTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/commit-tx")
	err = tx.Commit(commitCtx)
	commitTask.End()
	return err
}

func (qs *QuadStore) indexNode(ctx context.Context, tx kv.Tx, p *proto.Primitive, val quad.Value) error {
	var err error
	if val == nil {
		unmarshalCtx, unmarshalTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/index-new-nodes/index-node/unmarshal-value")
		val, err = pquads.UnmarshalValue(unmarshalCtx, p.Value)
		unmarshalTask.End()
		if err != nil {
			return err
		}
	}
	hash := quad.HashOf(val)
	key := bucketKeyForHash(hash)
	putCtx, putTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/index-new-nodes/index-node/put-value-key")
	err = tx.Put(putCtx, key, uint64toBytes(p.ID))
	putTask.End()
	if err != nil {
		return err
	}
	if iri, ok := val.(quad.IRI); ok {
		qs.valueLRU.Put(string(iri), p.ID)
	}
	logCtx, logTask := trace.NewTask(ctx, "cayley/kv/apply-deltas/apply-add-deltas/inc-nodes/index-new-nodes/index-node/add-log")
	err = qs.addToLog(logCtx, tx, p)
	logTask.End()
	return err
}

func (qs *QuadStore) indexLinks(ctx context.Context, tx kv.Tx, cache *metaCache, links []*proto.Primitive, newIDs map[uint64]struct{}) error {
	for _, p := range links {
		if err := qs.indexLink(ctx, tx, p, newIDs); err != nil {
			return err
		}
	}
	_, err := qs.incMetaInt(ctx, tx, cache, "size", int64(len(links)))
	return err
}

func (qs *QuadStore) indexLink(ctx context.Context, tx kv.Tx, p *proto.Primitive, newIDs map[uint64]struct{}) error {
	var err error
	qs.indexes.RLock()
	all := qs.indexes.all
	qs.indexes.RUnlock()
	for _, ind := range all {
		// A posting list is empty on disk when any of its index-key directions
		// resolves to a node minted in this transaction: no prior quad could
		// reference that node, so the flush can blind-write without a read.
		fresh := false
		for _, d := range ind.Dirs {
			if _, ok := newIDs[p.GetDirection(d)]; ok {
				fresh = true
				break
			}
		}
		err = qs.addToMapBucket(tx, ind.KeyFor(p), p.ID, fresh)
		if err != nil {
			return err
		}
	}
	err = qs.indexSchema(tx, p)
	if err != nil {
		return err
	}
	return qs.addToLog(ctx, tx, p)
}

func (qs *QuadStore) markAsDead(ctx context.Context, tx kv.Tx, p *proto.Primitive) error {
	p.Deleted = true
	// TODO(barakmich): Add tombstone?
	return qs.addToLog(ctx, tx, p)
}

func (qs *QuadStore) markLinksDead(ctx context.Context, tx kv.Tx, cache *metaCache, links []*proto.Primitive) error {
	for _, p := range links {
		if err := qs.markAsDead(ctx, tx, p); err != nil {
			return err
		}
	}
	_, err := qs.incMetaInt(ctx, tx, cache, "size", -int64(len(links)))
	return err
}

func (qs *QuadStore) getBucketIndexes(ctx context.Context, tx kv.Tx, keys []kv.Key) ([][]uint64, error) {
	vals, err := tx.GetBatch(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([][]uint64, len(keys))
	for i, v := range vals {
		if len(v) == 0 {
			continue
		}
		ind, err := decodeIndex(v)
		if err != nil {
			return out, err
		}
		out[i] = ind
	}
	return out, nil
}

func countIndex(b []byte) (int64, error) {
	var cnt int64
	for len(b) > 0 {
		_, n := binary.Uvarint(b)
		if n == 0 {
			return 0, io.ErrUnexpectedEOF
		} else if n < 0 {
			return 0, errors.New("varint: overflow")
		}
		cnt++
		b = b[n:]
	}
	return cnt, nil
}

func decodeIndex(b []byte) ([]uint64, error) {
	var out []uint64
	for len(b) > 0 {
		v, n := binary.Uvarint(b)
		if n == 0 {
			return out, io.ErrUnexpectedEOF
		} else if n < 0 {
			return out, errors.New("varint: overflow")
		}
		out = append(out, v)
		b = b[n:]
	}
	return out, nil
}

func appendIndex(bytelist []byte, l []uint64) []byte {
	b := make([]byte, len(bytelist)+(binary.MaxVarintLen64*len(l)))
	copy(b[:len(bytelist)], bytelist)
	off := len(bytelist)
	for _, x := range l {
		n := binary.PutUvarint(b[off:], x)
		off += n
	}
	return b[:off]
}

func (qs *QuadStore) bestUnique() ([]QuadIndex, error) {
	qs.indexes.RLock()
	ind := qs.indexes.exists
	qs.indexes.RUnlock()
	if len(ind) != 0 {
		return ind, nil
	}
	qs.indexes.Lock()
	defer qs.indexes.Unlock()
	if len(qs.indexes.exists) != 0 {
		return qs.indexes.exists, nil
	}
	for _, in := range qs.indexes.all {
		if in.Unique {
			if clog.V(2) {
				clog.Infof("using unique index: %v", in.Dirs)
			}
			qs.indexes.exists = []QuadIndex{in}
			return qs.indexes.exists, nil
		}
	}
	// TODO: find best combination of indexes
	inds := qs.indexes.all
	if len(inds) == 0 {
		return nil, fmt.Errorf("no indexes defined")
	}
	if clog.V(2) {
		clog.Infof("using index intersection: %v", inds)
	}
	qs.indexes.exists = inds
	return qs.indexes.exists, nil
}

func hasDir(dirs []quad.Direction, d quad.Direction) bool {
	return slices.Contains(dirs, d)
}

func (qs *QuadStore) bestIndexes(dirs []quad.Direction) []QuadIndex {
	qs.indexes.RLock()
	all := qs.indexes.all
	qs.indexes.RUnlock()
	var (
		max  int // more specific index is better
		best QuadIndex
	)
	for _, ind := range all {
		if len(ind.Dirs) < len(dirs) {
			continue // TODO(dennwc): allow intersecting indexes
		}
		match := 0
		for i, d := range ind.Dirs {
			if i >= len(dirs) || !hasDir(dirs, d) {
				break
			}
			match++
		}
		if match == len(dirs) {
			// exact index match
			return []QuadIndex{ind}
		}
		if match > 0 && match > max {
			best = ind
			max = match
		}
	}
	if max == 0 {
		return nil
	}
	// TODO(dennwc): intersect with some other index
	return []QuadIndex{best}
}

func (qs *QuadStore) hasPrimitive(ctx context.Context, tx kv.Tx, p *proto.Primitive, get bool) (*proto.Primitive, error) {
	dirs := make([]quad.Direction, 0, len(quad.Directions))
	for _, dir := range quad.Directions {
		if p.GetDirection(dir) == 0 {
			continue
		}
		dirs = append(dirs, dir)
	}
	inds, err := qs.bestUnique()
	if err != nil {
		return nil, err
	}
	unique := len(inds) != 0 && inds[0].Unique
	if !unique {
		if best := qs.bestIndexes(dirs); len(best) != 0 {
			inds = best
		}
	}
	keys := make([]kv.Key, len(inds))
	for i, in := range inds {
		keys[i] = in.KeyFor(p)
	}
	lists, err := qs.getBucketIndexes(ctx, tx, keys)
	if err != nil {
		return nil, err
	}
	var options []uint64
	for len(lists) > 0 {
		if len(lists) == 1 {
			options = lists[0]
			break
		}
		a, b := lists[0], lists[1]
		lists = lists[1:]
		a = intersectSortedUint64(a, b)
		lists[0] = a
	}
	if !get && unique {
		return p, nil
	}
	for i := len(options) - 1; i >= 0; i-- {
		// TODO: batch
		prim, err := qs.getPrimitiveFromLog(ctx, tx, options[i])
		if err != nil {
			return nil, err
		}
		if prim.Deleted {
			continue
		}
		if prim.IsSameLink(p) {
			return prim, nil
		}
	}
	return nil, nil
}

func intersectSortedUint64(a, b []uint64) []uint64 {
	var c []uint64
	boff := 0
outer:
	for _, x := range a {
		for {
			if boff >= len(b) {
				break outer
			}
			if x > b[boff] {
				boff++
				continue
			}
			if x < b[boff] {
				break
			}
			if x == b[boff] {
				c = append(c, x)
				boff++
				break
			}
		}
	}
	return c
}

// indexPosting is a pending index posting list buffered for one flush. fresh is
// true only when every buffered add proved the on-disk list empty (an index key
// direction resolved to a node minted in this transaction), so the flush can
// blind-write it and skip the read-modify-write read.
type indexPosting struct {
	ids   []uint64
	fresh bool
}

func (qs *QuadStore) addToMapBucket(tx kv.Tx, key kv.Key, value uint64, fresh bool) error {
	if len(key) != 2 {
		return fmt.Errorf("trying to add to map bucket with invalid key: %v", key)
	}
	b, k := key[0], key[1]
	if len(k) == 0 {
		return fmt.Errorf("trying to add to map bucket %s with key 0", b)
	}
	if qs.mapBucket == nil {
		qs.mapBucket = make(map[string]map[string]*indexPosting)
	}
	bucket := string(b)
	m, ok := qs.mapBucket[bucket]
	if !ok {
		m = make(map[string]*indexPosting)
		qs.mapBucket[bucket] = m
	}
	e, ok := m[string(k)]
	if !ok {
		e = &indexPosting{fresh: fresh}
		m[string(k)] = e
	} else {
		// Only skip the read when every add agrees the list is empty.
		e.fresh = e.fresh && fresh
	}
	e.ids = append(e.ids, value)
	return nil
}

func (qs *QuadStore) flushMapBucket(ctx context.Context, tx kv.Tx) error {
	bs := make([]string, 0, len(qs.mapBucket))
	for k := range qs.mapBucket {
		bs = append(bs, k)
	}
	sort.Strings(bs)
	for _, bucket := range bs {
		m := qs.mapBucket[bucket]
		if len(m) == 0 {
			continue
		}
		b := kv.Key{[]byte(bucket)}
		// Fresh postings blind-write; the rest read-modify-write in one batch.
		freshKeys := make([]kv.Key, 0, len(m))
		mergeKeys := make([]kv.Key, 0, len(m))
		for k, e := range m {
			key := b.AppendBytes([]byte(k))
			if e.fresh {
				freshKeys = append(freshKeys, key)
			} else {
				mergeKeys = append(mergeKeys, key)
			}
		}
		sort.Sort(kv.ByKey(freshKeys))
		for _, k := range freshKeys {
			buf := appendIndex(nil, m[string(k[1])].ids)
			if err := tx.Put(ctx, k, buf); err != nil {
				return err
			}
		}
		if len(mergeKeys) == 0 {
			continue
		}
		sort.Sort(kv.ByKey(mergeKeys))
		vals, err := tx.GetBatch(ctx, mergeKeys)
		if err != nil {
			return err
		}
		for i, k := range mergeKeys {
			buf := appendIndex(vals[i], m[string(k[1])].ids)
			if err := tx.Put(ctx, k, buf); err != nil {
				return err
			}
		}
	}
	qs.mapBucket = nil
	return nil
}

func (qs *QuadStore) indexSchema(tx kv.Tx, p *proto.Primitive) error {
	return nil
}

func (qs *QuadStore) addToLog(ctx context.Context, tx kv.Tx, p *proto.Primitive) error {
	buf, err := p.MarshalVT()
	if err != nil {
		return err
	}
	if err := tx.Put(ctx, logIndex.AppendBytes(uint64KeyBytesBase10(p.ID)), buf); err != nil {
		return err
	}
	return nil
}

func createNodePrimitive(v quad.Value) (*proto.Primitive, error) {
	p := &proto.Primitive{}
	b, err := pquads.MarshalValue(v)
	if err != nil {
		return p, err
	}
	p.Value = b
	return p, nil
}

func (qs *QuadStore) resolveQuadValue(ctx context.Context, tx kv.Tx, v quad.Value) (uint64, error) {
	out, err := qs.resolveQuadValues(ctx, tx, []quad.Value{v})
	if err != nil {
		return 0, err
	}
	return out[0], nil
}

func bucketKeyForVal(v quad.Value) kv.Key {
	hash := refs.HashOf(v)
	return bucketKeyForHash(hash[:])
}

func bucketKeyForHash(h []byte) kv.Key {
	hashB58 := b58.Encode(h)
	return bucketForVal().AppendBytes([]byte(hashB58))
}

func bucketKeyForHashRefs(h refs.ValueHash) kv.Key {
	hashB58 := b58.Encode(h[:])
	return bucketForValRefs().AppendBytes([]byte(hashB58))
}

func quadHashOf(q quad.Quad) refs.QuadHash {
	var qh refs.QuadHash
	for _, dir := range quad.Directions {
		if v := q.Get(dir); v != nil {
			qh.Set(dir, refs.HashOf(v))
		}
	}
	return qh
}

func (qs *QuadStore) rememberQuadExists(qh refs.QuadHash) {
	if len(qs.quadExists) >= 2000 {
		clear(qs.quadExists)
	}
	qs.quadExists[qh] = struct{}{}
}

func (qs *QuadStore) resolveQuadValues(ctx context.Context, tx kv.Tx, vals []quad.Value) ([]uint64, error) {
	out := make([]uint64, len(vals))
	inds := make([]int, 0, len(vals))
	keys := make([]kv.Key, 0, len(vals))
	for i, v := range vals {
		if iri, ok := v.(quad.IRI); ok {
			if x, ok := qs.valueLRU.Get(string(iri)); ok {
				out[i] = x.(uint64)
				continue
			}
		} else if v == nil {
			continue
		}
		inds = append(inds, i)
		keys = append(keys, bucketKeyForVal(v))
	}
	if len(keys) == 0 {
		return out, nil
	}
	resp, err := tx.GetBatch(ctx, keys)
	if err != nil {
		return out, err
	}
	for i, b := range resp {
		if len(b) == 0 {
			continue
		}
		ind := inds[i]
		out[ind], _ = binary.Uvarint(b)
		if iri, ok := vals[ind].(quad.IRI); ok && out[ind] != 0 {
			qs.valueLRU.Put(string(iri), out[ind])
		}
	}
	return out, nil
}

func uint64toBytes(x uint64) []byte {
	b := make([]byte, binary.MaxVarintLen64)
	return uint64toBytesAt(x, b)
}

func uint64toBytesAt(x uint64, bytes []byte) []byte {
	n := binary.PutUvarint(bytes, x)
	return bytes[:n]
}

func uint64KeyBytesBase10(n uint64) []byte {
	return []byte(strconv.FormatUint(n, 10))
}

func uint64KeyBytes(n uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], n)

	// base62 encoding
	var i big.Int
	i.SetBytes(data[:])
	s := i.Text(62)

	// if padding (lexographical sort) is needed:
	/*
		if padding {
			const digits = 11 // max digits for a uint64 number in base62
			prefix := digits - len(s)
			s = strings.Repeat("-", prefix) + s
		}
	*/

	return []byte(s)
}

func (qs *QuadStore) getPrimitivesFromLog(ctx context.Context, tx kv.Tx, keys []uint64) ([]*proto.Primitive, error) {
	bkeys := make([]kv.Key, len(keys))
	for i, k := range keys {
		bkeys[i] = logIndex.Append(kv.Key{uint64KeyBytesBase10(k)})
	}
	vals, err := tx.GetBatch(ctx, bkeys)
	if err != nil {
		return nil, err
	}
	out := make([]*proto.Primitive, len(keys))
	var last error
	for i, v := range vals {
		if v == nil {
			continue
		}
		p := &proto.Primitive{}
		if err = p.UnmarshalVT(v); err != nil {
			last = err
		} else {
			out[i] = p
		}
	}
	return out, last
}

func (qs *QuadStore) getPrimitiveFromLog(ctx context.Context, tx kv.Tx, k uint64) (*proto.Primitive, error) {
	out, err := qs.getPrimitivesFromLog(ctx, tx, []uint64{k})
	if err != nil {
		return nil, err
	} else if out[0] == nil {
		return nil, kv.ErrNotFound
	}
	return out[0], nil
}

type Int64Set []uint64

func (a Int64Set) Len() int           { return len(a) }
func (a Int64Set) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Int64Set) Less(i, j int) bool { return a[i] < a[j] }
