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
		qs.indexes.all = indexes
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
	qs.indexes.all = indexes
	return nil
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
	}
	return strings.Join(buckets, ",")
}
