package kv

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
	"github.com/pkg/errors"
)

// ErrIndexChangeInProgress is returned when a layout change is requested while
// a different one is still in progress.
var ErrIndexChangeInProgress = errors.New("kv: a different quad index change is in progress")

// BeginIndexChange records target as the layout to reach while the store stays
// open. A new index starts building: writes maintain it, lookups do not use it.
// A removed index starts dropping: neither writes nor lookups use it. An index
// in both layouts keeps its postings; a changed Unique flag or order applies
// at once. StepIndexChange then advances the change. Beginning the change
// already in progress does nothing; beginning a different one returns
// ErrIndexChangeInProgress. Target indexes must be complete.
func (qs *QuadStore) BeginIndexChange(ctx context.Context, target []QuadIndex) error {
	// Validate the target before touching the store.
	target, err := cloneQuadIndexes(target)
	if err != nil {
		return err
	}
	if err := requireCompleteIndexes(target); err != nil {
		return err
	}
	targetBuckets := make(map[string]struct{}, len(target))
	for _, index := range target {
		bucket := string(index.bucket()[0])
		if _, dup := targetBuckets[bucket]; dup {
			return errors.Errorf("quad indexes [%s] repeat posting bucket %s", quadIndexLayout(target), bucket)
		}
		targetBuckets[bucket] = struct{}{}
	}

	// Read the stored layout under the writer, as every write does.
	qs.writer.Lock()
	defer qs.writer.Unlock()
	tx, err := qs.db.Tx(ctx, true)
	if err != nil {
		return err
	}
	defer tx.Close()
	cur, err := qs.readChangeLayout(ctx, tx)
	if err != nil {
		return err
	}

	// A change in progress either is this one or blocks it.
	if changeInProgress(cur.indexes) {
		if CompareQuadIndexes(changeTarget(cur.indexes), target) {
			return nil
		}
		return errors.Wrapf(ErrIndexChangeInProgress, "stored [%s], requested [%s]", quadIndexLayout(cur.indexes), quadIndexLayout(target))
	}

	// Keep the postings of each index in both layouts, build the new ones,
	// and drop the rest after them.
	next := make([]QuadIndex, 0, len(target)+len(cur.indexes))
	var kept bool
	for _, index := range target {
		if slices.ContainsFunc(cur.indexes, func(c QuadIndex) bool { return CompareQuadDirections(c.Dirs, index.Dirs) }) {
			kept = true
		} else {
			index.State = QuadIndexState_BUILDING
		}
		next = append(next, index)
	}
	for _, index := range cur.indexes {
		if _, ok := targetBuckets[string(index.bucket()[0])]; !ok {
			index.State = QuadIndexState_DROPPING
			next = append(next, index)
		}
	}
	if CompareQuadIndexes(next, cur.indexes) {
		return nil
	}
	if !kept {
		return errors.Errorf("quad index change from [%s] to [%s] keeps no index to build from", quadIndexLayout(cur.indexes), quadIndexLayout(target))
	}

	// Record the change and serve from the new layout.
	if err := qs.commitLayout(ctx, tx, &indexLayout{indexes: next}); err != nil {
		return err
	}
	qs.setLayout(next)
	return nil
}

// StepIndexChange advances the layout change in progress in one transaction
// and reports whether the store has reached its target. A build step indexes
// up to max live links, read from the first complete index's postings in key
// order after the stored cursor; when the postings run out, the building
// indexes become complete. Builds finish before drops. A drop step deletes up
// to max postings of the first dropping index, and removes the index once its
// postings are gone. A step depends only on the store's contents and max, so
// stores stepped alike hold the same bytes. Without a change in progress it
// does nothing and reports done.
func (qs *QuadStore) StepIndexChange(ctx context.Context, max int) (bool, error) {
	// A step must make progress.
	if max <= 0 {
		return false, errors.New("quad index change step requires a positive size")
	}

	// Read the stored layout and cursor under the writer, as every write does.
	qs.writer.Lock()
	defer qs.writer.Unlock()
	tx, err := qs.db.Tx(ctx, true)
	if err != nil {
		return false, err
	}
	defer tx.Close()
	cur, err := qs.readChangeLayout(ctx, tx)
	if err != nil {
		return false, err
	}
	if !changeInProgress(cur.indexes) {
		return true, nil
	}

	// Build before dropping, so a build always has its source.
	var next *indexLayout
	if slices.ContainsFunc(cur.indexes, isBuilding) {
		next, err = qs.buildStep(ctx, tx, cur, max)
	} else {
		next, err = qs.dropStep(ctx, tx, cur, max)
	}
	if err != nil {
		return false, err
	}

	// Record the advanced layout, then serve from it.
	if err := qs.commitLayout(ctx, tx, next); err != nil {
		return false, err
	}
	if !CompareQuadIndexes(next.indexes, cur.indexes) {
		qs.setLayout(next.indexes)
	}
	return !changeInProgress(next.indexes), nil
}

// buildStep indexes up to max links of the source postings after the cursor
// into every building index, and returns the advanced layout.
func (qs *QuadStore) buildStep(ctx context.Context, tx kv.Tx, cur *indexLayout, max int) (*indexLayout, error) {
	// Collect the next link IDs from the first complete index.
	src := cur.indexes[slices.IndexFunc(cur.indexes, func(index QuadIndex) bool {
		return index.State == QuadIndexState_COMPLETE
	})]
	ids, cursorKey, cursorID, finished, err := scanBuildSource(ctx, tx, src, cur.cursorKey, cur.cursorID, max)
	if err != nil {
		return nil, err
	}

	// Group the live links by their posting in each building index. The
	// source key lacks directions a building index may hold, so read the
	// primitives.
	prims, err := qs.getPrimitivesFromLog(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	postings := make(map[postingRef][]uint64)
	for _, index := range cur.indexes {
		if !isBuilding(index) {
			continue
		}
		for _, p := range prims {
			if p == nil || p.Deleted || p.IsNode() {
				continue
			}
			key := index.KeyFor(p)
			ref := postingRef{bucket: string(key[0]), key: string(key[1])}
			postings[ref] = append(postings[ref], p.ID)
		}
	}

	// Merge each group into its posting. A write may already have added a
	// link the step reaches, so merge rather than overwrite.
	if err := mergePostings(ctx, tx, postings); err != nil {
		return nil, err
	}

	// Advance the cursor, or complete the building indexes.
	next := &indexLayout{indexes: slices.Clone(cur.indexes), cursorKey: cursorKey, cursorID: cursorID}
	if finished {
		next.cursorKey, next.cursorID = nil, 0
		for i := range next.indexes {
			if isBuilding(next.indexes[i]) {
				next.indexes[i].State = QuadIndexState_COMPLETE
			}
		}
	}
	return next, nil
}

// scanBuildSource returns up to max link IDs from src's postings, in key order
// and then ID order, strictly after the cursor. It returns the new cursor and
// whether the postings ran out.
func scanBuildSource(ctx context.Context, tx kv.Tx, src QuadIndex, afterKey []byte, afterID uint64, max int) ([]uint64, []byte, uint64, bool, error) {
	// Seek to the cursor's posting, which may hold links past the cursor ID.
	it := tx.Scan(ctx, options.WithPrefixKV(src.bucket()))
	defer it.Close()
	var valid bool
	if len(afterKey) == 0 {
		valid = it.Next(ctx)
	} else {
		valid = kv.Seek(ctx, it, src.bucket().AppendBytes(afterKey))
	}

	// Gather IDs until max, remembering the last one taken.
	ids := make([]uint64, 0, max)
	var (
		lastKey []byte
		lastID  uint64
	)
	for ; valid; valid = it.Next(ctx) {
		key := it.Key()
		if len(key) != 2 {
			return nil, nil, 0, false, errors.New("kv: unexpected index key shape")
		}
		posted, err := decodeIndex(it.Val())
		if err != nil {
			return nil, nil, 0, false, err
		}
		postingKey := bytes.Clone(key[1])
		resume := bytes.Equal(postingKey, afterKey)
		for _, id := range posted {
			if resume && id <= afterID {
				continue
			}
			if len(ids) == max {
				return ids, lastKey, lastID, false, nil
			}
			ids = append(ids, id)
			lastKey, lastID = postingKey, id
		}
	}
	if err := it.Err(); err != nil {
		return nil, nil, 0, false, err
	}
	return ids, nil, 0, true, nil
}

// postingRef names one posting by its bucket and key.
type postingRef struct {
	bucket, key string
}

// mergePostings adds each group of link IDs to its posting, keeping every
// posting ascending and free of repeats. Keys are written in sorted order.
func mergePostings(ctx context.Context, tx kv.Tx, postings map[postingRef][]uint64) error {
	// Read every posting the step touches in one batch.
	refs := make([]postingRef, 0, len(postings))
	for ref := range postings {
		refs = append(refs, ref)
	}
	slices.SortFunc(refs, func(a, b postingRef) int {
		if c := strings.Compare(a.bucket, b.bucket); c != 0 {
			return c
		}
		return strings.Compare(a.key, b.key)
	})
	keys := make([]kv.Key, len(refs))
	for i, ref := range refs {
		keys[i] = kv.Key{[]byte(ref.bucket), []byte(ref.key)}
	}
	vals, err := tx.GetBatch(ctx, keys)
	if err != nil {
		return err
	}

	// Write the sorted union of each stored posting and its new IDs.
	for i, key := range keys {
		stored, err := decodeIndex(vals[i])
		if err != nil {
			return err
		}
		merged := slices.Concat(stored, postings[refs[i]])
		slices.Sort(merged)
		merged = slices.Compact(merged)
		if len(merged) == len(stored) {
			continue
		}
		if err := tx.Put(ctx, key, appendIndex(nil, merged)); err != nil {
			return err
		}
	}
	return nil
}

// dropStep deletes up to max postings of the first dropping index, removing
// the index from the layout once none remain.
func (qs *QuadStore) dropStep(ctx context.Context, tx kv.Tx, cur *indexLayout, max int) (*indexLayout, error) {
	// Collect one posting past max, to learn whether this step empties the
	// index, before deleting any, so the scan never sees its own deletes.
	at := slices.IndexFunc(cur.indexes, func(index QuadIndex) bool {
		return index.State == QuadIndexState_DROPPING
	})
	it := tx.Scan(ctx, options.WithPrefixKV(cur.indexes[at].bucket()))
	var keys []kv.Key
	for len(keys) <= max && it.Next(ctx) {
		keys = append(keys, it.Key().Clone())
	}
	scanErr := it.Err()
	closeErr := it.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	// Delete up to max of them.
	emptied := len(keys) <= max
	keys = keys[:min(len(keys), max)]
	for _, key := range keys {
		if err := tx.Del(ctx, key); err != nil {
			return nil, err
		}
	}

	// Forget the index once its last posting is gone.
	next := &indexLayout{indexes: slices.Clone(cur.indexes)}
	if emptied {
		next.indexes = slices.Delete(next.indexes, at, at+1)
	}
	return next, nil
}

// readChangeLayout reads the stored layout and refuses one this handle does
// not serve, which another handle changed after this one opened.
func (qs *QuadStore) readChangeLayout(ctx context.Context, tx kv.Tx) (*indexLayout, error) {
	cur, err := readIndexLayout(ctx, tx)
	if err != nil {
		return nil, err
	}
	if active := qs.Layout(); !CompareQuadIndexes(cur.indexes, active) {
		return nil, errors.Errorf("stored quad indexes [%s] differ from this handle's [%s]; reopen the store", quadIndexLayout(cur.indexes), quadIndexLayout(active))
	}
	return cur, nil
}

// commitLayout records the layout and commits the step. The record replaces
// any layout this handle meant to record with its first write. A finished
// change back to the default layout removes the record, as a store that never
// changed has none.
func (qs *QuadStore) commitLayout(ctx context.Context, tx kv.Tx, layout *indexLayout) error {
	// Record the layout, or remove the record the default layout needs none of.
	if len(layout.cursorKey) == 0 && CompareQuadIndexes(layout.indexes, DefaultQuadIndexes) {
		if err := tx.Del(ctx, kv.Key{keyMetaIndexes}); err != nil {
			return err
		}
	} else {
		data, err := encodeIndexLayout(layout)
		if err != nil {
			return err
		}
		if err := tx.Put(ctx, kv.Key{keyMetaIndexes}, data); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	qs.indexMeta = nil
	return nil
}

// isBuilding reports whether the index is being built.
func isBuilding(index QuadIndex) bool {
	return index.State == QuadIndexState_BUILDING
}

// changeInProgress reports whether any index is building or dropping.
func changeInProgress(indexes []QuadIndex) bool {
	return slices.ContainsFunc(indexes, func(index QuadIndex) bool {
		return index.State != QuadIndexState_COMPLETE
	})
}

// changeTarget returns the layout a change in progress reaches: its indexes
// that are not dropping, all complete.
func changeTarget(indexes []QuadIndex) []QuadIndex {
	var out []QuadIndex
	for _, index := range indexes {
		if index.State != QuadIndexState_DROPPING {
			index.State = QuadIndexState_COMPLETE
			out = append(out, index)
		}
	}
	return out
}
