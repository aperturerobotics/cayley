package kv

import (
	"context"
	"slices"
	"strings"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
	"github.com/pkg/errors"
)

// cloneQuadIndexes validates a posting layout and detaches its mutable slices.
func cloneQuadIndexes(indexes []QuadIndex) ([]QuadIndex, error) {
	// Empty direction lists escape to an empty KV prefix and cannot name a posting bucket.
	if len(indexes) == 0 {
		return nil, errors.New("quad indexes require at least one posting index")
	}
	for _, index := range indexes {
		if len(index.Dirs) == 0 {
			return nil, errors.New("quad indexes require at least one direction per index")
		}
	}
	if _, err := encodeQuadIndexes(indexes); err != nil {
		return nil, err
	}

	// Keep both the index list and each direction list private to the graph.
	indexes = slices.Clone(indexes)
	for i := range indexes {
		indexes[i].Dirs = slices.Clone(indexes[i].Dirs)
	}
	return indexes, nil
}

// keyIndexRebuild prevents graph access while postings are being replaced.
var keyIndexRebuild = metaBucket.AppendBytes([]byte("reindex"))

// checkIndexRebuild refuses access to postings left by an interrupted rebuild.
func (qs *QuadStore) checkIndexRebuild(ctx context.Context, opt graph.Options) error {
	// Check rebuild state without retaining a read transaction during metadata reads.
	var incomplete bool
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		// Read the completion marker without changing the graph.
		_, err := tx.Get(ctx, keyIndexRebuild)
		if err == kv.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		incomplete = true
		return nil
	})
	if err != nil || !incomplete {
		return err
	}

	// Report the stored and requested layouts when the caller selects indexes.
	stored, err := qs.readIndexesMeta(ctx)
	if err != nil {
		return err
	}
	if requested, ok := opt[OptQuadIndexes].([]QuadIndex); ok {
		return errors.Errorf("quad index rebuild incomplete; stored [%s], requested [%s]; rerun the application's offline reindex command before opening", quadIndexLayout(stored), quadIndexLayout(requested))
	}
	return errors.New("quad index rebuild incomplete; rerun the application's offline reindex command before opening")
}

// selectQuadIndexes checks nonempty graphs and selects an empty graph's layout.
func (qs *QuadStore) selectQuadIndexes(ctx context.Context, indexes []QuadIndex) error {
	// Compare the requested layout with the durable layout, including the legacy default.
	stored, err := qs.readIndexesMeta(ctx)
	if err != nil {
		return err
	}
	if CompareQuadIndexes(stored, indexes) {
		qs.setLayout(indexes)
		return nil
	}

	// Existing primitives require a manual rebuild before the layout can change.
	nonempty, err := qs.hasPrimitiveLog(ctx)
	if err != nil {
		return err
	}
	if nonempty {
		return errors.Errorf("stored quad indexes [%s] differ from requested [%s]; run the application's offline reindex command before opening", quadIndexLayout(stored), quadIndexLayout(indexes))
	}

	// Record an empty store's layout with its first mutation, so read-only opens work.
	qs.indexMeta, err = encodeQuadIndexes(indexes)
	if err != nil {
		return err
	}
	qs.setLayout(indexes)
	return nil
}

// trustQuadIndexes takes the caller's copy of the stored layout without
// reading the store. A non-default layout is recorded by the first write, so
// an empty store holds it from its first primitive.
func (qs *QuadStore) trustQuadIndexes(indexes []QuadIndex) error {
	// Validate the layout and keep a private copy.
	indexes, err := cloneQuadIndexes(indexes)
	if err != nil {
		return err
	}

	// The default layout is what an absent record means.
	if !CompareQuadIndexes(indexes, DefaultQuadIndexes) {
		qs.indexMeta, err = encodeQuadIndexes(indexes)
		if err != nil {
			return err
		}
	}
	qs.setLayout(indexes)
	return nil
}

// requireCompleteIndexes refuses a requested layout holding an index that is
// building or dropping; only a layout change records those states.
func requireCompleteIndexes(indexes []QuadIndex) error {
	for _, index := range indexes {
		if index.State != QuadIndexState_COMPLETE {
			return errors.Errorf("requested quad index %s is %s; request complete indexes", index.bucket()[0], index.State)
		}
	}
	return nil
}

// setLayout replaces the active layout and the views derived from it.
func (qs *QuadStore) setLayout(layout []QuadIndex) {
	// Split the layout into the indexes that serve and those writes maintain.
	var all, building []QuadIndex
	for _, index := range layout {
		switch index.State {
		case QuadIndexState_COMPLETE:
			all = append(all, index)
		case QuadIndexState_BUILDING:
			building = append(building, index)
		}
	}

	// Publish the views and drop the cached exact lookup indexes.
	qs.indexes.Lock()
	qs.indexes.layout = layout
	qs.indexes.all = all
	qs.indexes.building = building
	qs.indexes.exists = nil
	qs.indexes.Unlock()
}

// Layout returns a copy of every index in the active layout with its state.
func (qs *QuadStore) Layout() []QuadIndex {
	// Copy the layout and its direction slices, which the caller may change.
	qs.indexes.RLock()
	layout := qs.indexes.layout
	qs.indexes.RUnlock()
	out := slices.Clone(layout)
	for i := range out {
		out[i].Dirs = slices.Clone(out[i].Dirs)
	}
	return out
}

// maintainedIndexes returns the indexes writes keep current: the complete
// indexes and those being built.
func (qs *QuadStore) maintainedIndexes() []QuadIndex {
	qs.indexes.RLock()
	defer qs.indexes.RUnlock()
	if len(qs.indexes.building) == 0 {
		return qs.indexes.all
	}
	return slices.Concat(qs.indexes.all, qs.indexes.building)
}

// hasPrimitiveLog checks one log entry without walking the graph.
func (qs *QuadStore) hasPrimitiveLog(ctx context.Context) (bool, error) {
	// Inspect only the first primitive, including deleted entries.
	var nonempty bool
	err := kv.View(ctx, qs.db, func(tx kv.Tx) error {
		// Check presence and release the scan before returning its error.
		it := tx.Scan(ctx, options.WithPrefixKV(logIndex))
		nonempty = it.Next(ctx)
		scanErr := it.Err()
		closeErr := it.Close()
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	return nonempty, err
}

// quadIndexLayout describes posting buckets and their uniqueness constraints.
func quadIndexLayout(indexes []QuadIndex) string {
	// Retain layout order so the diagnostic describes the exact stored configuration.
	buckets := make([]string, len(indexes))
	for i, index := range indexes {
		buckets[i] = string(index.bucket()[0])
		if index.Unique {
			buckets[i] += " (unique)"
		}
		if index.State != QuadIndexState_COMPLETE {
			buckets[i] += " (" + strings.ToLower(index.State.String()) + ")"
		}
	}
	return strings.Join(buckets, ",")
}
