package kv

import (
	"context"
	"maps"
	"slices"

	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
	"github.com/pkg/errors"
)

// indexRebuildBatchSize bounds keys and primitives held by a rebuild transaction.
const indexRebuildBatchSize = 4096

// IndexRebuilder regenerates quad postings from the durable primitive log.
// The database must remain closed to graph readers and writers until Rebuild
// succeeds. A failed or interrupted rebuild can be rerun from the beginning.
// Node values, reference counts, graph size, and primitive IDs are preserved.
type IndexRebuilder struct {
	// db holds the graph's primitive log and postings.
	db kv.KV
	// indexes is the complete posting layout to regenerate.
	indexes []QuadIndex
}

// NewIndexRebuilder validates and copies the desired posting layout.
func NewIndexRebuilder(db kv.KV, indexes []QuadIndex) (*IndexRebuilder, error) {
	// Require an independent, valid layout before removing any stored posting.
	indexes, err := cloneQuadIndexes(indexes)
	if err != nil {
		return nil, err
	}
	return &IndexRebuilder{db: db, indexes: indexes}, nil
}

// Rebuild clears old and current postings, reconstructs live quad indexes in
// bounded transactions, and records the resulting layout. It returns the
// number of live quads indexed by committed transactions, including on error.
func (r *IndexRebuilder) Rebuild(ctx context.Context) (uint64, error) {
	// Honor cancellation before removing any posting.
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	// Discover every posting bucket that must be cleared.
	qs := newQuadStore(r.db)
	old, err := qs.readIndexesMeta(ctx)
	if err != nil {
		return 0, err
	}
	old, err = cloneQuadIndexes(old)
	if err != nil {
		return 0, errors.Wrap(err, "invalid stored quad indexes")
	}
	buckets := make(map[string]kv.Key, len(old)+len(r.indexes))
	for _, index := range slices.Concat(old, r.indexes) {
		key := index.bucket()
		buckets[string(key[0])] = key
	}

	// Require a usable primitive horizon before changing any posting.
	horizon, err := qs.getMetaInt(ctx, "horizon")
	if err == ErrNoBucket {
		nonempty, logErr := qs.hasPrimitiveLog(ctx)
		if logErr != nil {
			return 0, logErr
		}
		if nonempty {
			return 0, errors.New("missing primitive horizon for nonempty log")
		}
	}
	if err != nil && err != ErrNoBucket {
		return 0, err
	}
	if horizon < 0 {
		return 0, errors.New("invalid negative primitive horizon")
	}

	// Mark postings unavailable before committing any destructive batch.
	if err := kv.Update(ctx, r.db, func(tx kv.Tx) error {
		return tx.Put(ctx, keyIndexRebuild, []byte{1})
	}); err != nil {
		return 0, err
	}

	// Clear postings without changing the authoritative primitive log.
	for _, name := range slices.Sorted(maps.Keys(buckets)) {
		if err := r.clearBucket(ctx, buckets[name]); err != nil {
			return 0, err
		}
	}
	qs.indexes.all = r.indexes

	// Read primitives in numeric ID order so every posting remains sorted.
	var count uint64
	for start := uint64(1); start <= uint64(horizon); {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		length := min(uint64(indexRebuildBatchSize), uint64(horizon)-start+1)
		ids := make([]uint64, int(length))
		for i := range ids {
			ids[i] = start + uint64(i)
		}
		n, err := r.rebuildBatch(ctx, qs, ids)
		count += n
		if err != nil {
			return count, err
		}
		start += length
	}

	// Publish the completed layout and restore graph access in one commit.
	data, err := encodeQuadIndexes(r.indexes)
	if err != nil {
		return count, err
	}
	return count, kv.Update(ctx, r.db, func(tx kv.Tx) error {
		if err := tx.Put(ctx, kv.Key{keyMetaIndexes}, data); err != nil {
			return err
		}
		return tx.Del(ctx, keyIndexRebuild)
	})
}

// clearBucket removes posting keys in bounded transactions.
func (r *IndexRebuilder) clearBucket(ctx context.Context, bucket kv.Key) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Collect and remove one batch after closing its scan.
		var count int
		err := kv.Update(ctx, r.db, func(tx kv.Tx) error {
			// Retain a bounded set of posting keys before closing the scan.
			it := tx.Scan(ctx, options.WithPrefixKV(bucket))
			keys := make([]kv.Key, 0, indexRebuildBatchSize)
			for len(keys) < indexRebuildBatchSize && it.Next(ctx) {
				keys = append(keys, it.Key().Clone())
			}
			err := it.Err()
			closeErr := it.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}

			// Delete only the keys gathered from this posting bucket.
			count = len(keys)
			for _, key := range keys {
				if err := tx.Del(ctx, key); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if count < indexRebuildBatchSize {
			return nil
		}
	}
}

// rebuildBatch indexes a bounded set of primitive IDs in numeric order.
func (r *IndexRebuilder) rebuildBatch(ctx context.Context, qs *QuadStore, ids []uint64) (uint64, error) {
	// Read live quad primitives under the transaction that writes their postings.
	var count uint64
	err := kv.Update(ctx, r.db, func(tx kv.Tx) error {
		// Reuse the graph's log decoder, including missing and deleted entries.
		primitives, err := qs.getPrimitivesFromLog(ctx, tx, ids)
		if err != nil {
			return err
		}

		// Preserve ascending primitive IDs without changing nodes or counters.
		for _, p := range primitives {
			if p == nil || p.Deleted || p.IsNode() {
				continue
			}
			for _, index := range r.indexes {
				if err := qs.addToMapBucket(tx, index.KeyFor(p), p.ID, false); err != nil {
					return err
				}
			}
			count++
		}
		return qs.flushMapBucket(ctx, tx)
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
