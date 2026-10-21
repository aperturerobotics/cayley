package kv

import (
	"bytes"
	"context"
	"encoding/binary"
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
// succeeds. A failed or interrupted rebuild is rerun with the same layout and
// resumes after its last committed batch. Node values, reference counts, graph
// size, and primitive IDs are preserved.
type IndexRebuilder struct {
	// db holds the graph's primitive log and postings.
	db kv.KV
	// indexes is the complete posting layout to regenerate.
	indexes []QuadIndex
	// layout is the encoded layout, recorded in the rebuild marker.
	layout []byte
	// written holds the posting keys written by the rebuild, so a key it
	// does not hold is absent and is written without reading it first.
	written *keyFilter
	// progress, when set, receives the next primitive ID after each batch.
	progress func(next, horizon uint64)
}

// NewIndexRebuilder validates and copies the desired posting layout.
func NewIndexRebuilder(db kv.KV, indexes []QuadIndex) (*IndexRebuilder, error) {
	// Require an independent, valid layout before removing any stored posting.
	indexes, err := cloneQuadIndexes(indexes)
	if err != nil {
		return nil, err
	}
	if err := requireCompleteIndexes(indexes); err != nil {
		return nil, err
	}
	layout, err := encodeQuadIndexes(indexes)
	if err != nil {
		return nil, err
	}
	return &IndexRebuilder{db: db, indexes: indexes, layout: layout}, nil
}

// SetProgress sets a function called after each committed batch with the next
// primitive ID to index and the primitive horizon.
func (r *IndexRebuilder) SetProgress(fn func(next, horizon uint64)) {
	r.progress = fn
}

// Rebuild clears old and current postings, reconstructs live quad indexes in
// bounded transactions, and records the resulting layout. Each batch records
// its successor in the rebuild marker, so rerunning an interrupted rebuild of
// the same layout skips the clear and the committed batches. It returns the
// number of live quads indexed by this call's committed transactions,
// including on error.
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

	// Resume after the last committed batch of an interrupted rebuild.
	start, err := r.readResume(ctx)
	if err != nil {
		return 0, err
	}
	r.written = newKeyFilter(uint64(horizon) * uint64(len(r.indexes)))
	if start != 0 {
		err = r.loadWritten(ctx)
	} else {
		start = 1
		err = r.clear(ctx, buckets)
	}
	if err != nil {
		return 0, err
	}
	qs.setLayout(r.indexes)

	// Read primitives in numeric ID order so every posting remains sorted.
	var count uint64
	for start <= uint64(horizon) {
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
		if r.progress != nil {
			r.progress(start, uint64(horizon))
		}
	}

	// Publish the completed layout and restore graph access in one commit.
	return count, kv.Update(ctx, r.db, func(tx kv.Tx) error {
		if err := tx.Put(ctx, kv.Key{keyMetaIndexes}, r.layout); err != nil {
			return err
		}
		return tx.Del(ctx, keyIndexRebuild)
	})
}

// readResume returns the next primitive ID recorded by an interrupted rebuild
// of this layout, or zero when the postings must be cleared first.
func (r *IndexRebuilder) readResume(ctx context.Context) (uint64, error) {
	var start uint64
	err := kv.View(ctx, r.db, func(tx kv.Tx) error {
		// An absent marker records a completed rebuild.
		val, err := tx.Get(ctx, keyIndexRebuild)
		if err == kv.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		// Resume only the layout this rebuilder writes.
		if len(val) > 8 && bytes.Equal(val[8:], r.layout) {
			start = binary.BigEndian.Uint64(val)
		}
		return nil
	})
	return start, err
}

// marker encodes the rebuild marker naming the next primitive ID to index.
// Zero records that the clear has not finished.
func (r *IndexRebuilder) marker(next uint64) []byte {
	return append(binary.BigEndian.AppendUint64(nil, next), r.layout...)
}

// clear marks postings unavailable, removes every posting bucket, and
// records that indexing starts at the first primitive.
func (r *IndexRebuilder) clear(ctx context.Context, buckets map[string]kv.Key) error {
	// Mark postings unavailable before committing any destructive batch.
	if err := kv.Update(ctx, r.db, func(tx kv.Tx) error {
		return tx.Put(ctx, keyIndexRebuild, r.marker(0))
	}); err != nil {
		return err
	}

	// Clear postings without changing the authoritative primitive log.
	for _, name := range slices.Sorted(maps.Keys(buckets)) {
		if err := r.clearBucket(ctx, buckets[name]); err != nil {
			return err
		}
	}
	return kv.Update(ctx, r.db, func(tx kv.Tx) error {
		return tx.Put(ctx, keyIndexRebuild, r.marker(1))
	})
}

// loadWritten records the posting keys committed before an interruption.
func (r *IndexRebuilder) loadWritten(ctx context.Context) error {
	return kv.View(ctx, r.db, func(tx kv.Tx) error {
		for _, index := range r.indexes {
			it := tx.Scan(ctx, options.WithPrefixKV(index.bucket()))
			for it.Next(ctx) {
				r.written.add(it.Key())
			}
			err := it.Err()
			if cerr := it.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// clearBucket removes posting keys in bounded transactions. Each batch seeks
// to the last key the previous batch deleted instead of rescanning the bucket,
// because stores that keep deletions until compaction would otherwise walk
// every earlier deletion again, making the clear quadratic in the bucket size.
func (r *IndexRebuilder) clearBucket(ctx context.Context, bucket kv.Key) error {
	var resume kv.Key
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Collect and remove one batch after closing its scan.
		var keys []kv.Key
		err := kv.Update(ctx, r.db, func(tx kv.Tx) error {
			// Retain a bounded set of posting keys before closing the scan.
			it := tx.Scan(ctx, options.WithPrefixKV(bucket))
			keys = make([]kv.Key, 0, indexRebuildBatchSize)
			var ok bool
			if resume == nil {
				ok = it.Next(ctx)
			} else {
				ok = kv.Seek(ctx, it, resume)
			}

			// Gather keys until the batch is full or the bucket ends.
			for ok {
				keys = append(keys, it.Key().Clone())
				if len(keys) == indexRebuildBatchSize {
					break
				}
				ok = it.Next(ctx)
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
		if len(keys) < indexRebuildBatchSize {
			return nil
		}
		resume = keys[len(keys)-1]
	}
}

// rebuildBatch indexes a bounded set of primitive IDs in numeric order and
// records the next ID in the rebuild marker. A posting key the rebuild has not
// written is absent, so it is written without reading it first.
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
				key := index.KeyFor(p)
				fresh := !r.written.has(key)
				r.written.add(key)
				if err := qs.addToMapBucket(tx, key, p.ID, fresh); err != nil {
					return err
				}
			}
			count++
		}
		if err := qs.flushMapBucket(ctx, tx); err != nil {
			return err
		}
		return tx.Put(ctx, keyIndexRebuild, r.marker(ids[len(ids)-1]+1))
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
