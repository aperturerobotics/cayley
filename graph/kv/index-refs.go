package kv

import (
	"context"
	"io"
	"slices"

	"github.com/aperturerobotics/cayley/kv"
	kvoptions "github.com/aperturerobotics/cayley/kv/options"
	"github.com/pkg/errors"
)

// IterateIndexPrefixNextRefs scans an index prefix and yields refs for the
// single remaining indexed direction directly from the index key.
func (qs *QuadStore) IterateIndexPrefixNextRefs(
	ctx context.Context,
	ind QuadIndex,
	vals []uint64,
	cb func(Int64Value, func() (bool, error)) error,
) error {
	return qs.IterateIndexPrefixNextRefsAfter(ctx, ind, vals, 0, cb)
}

// IterateIndexPrefixNextRefsAfter yields the remaining direction in posting-key
// order, seeking strictly past after when it is non-zero. It visits each
// distinct reference once, including when several quads share its posting.
// Returning io.EOF from cb stops the scan successfully.
func (qs *QuadStore) IterateIndexPrefixNextRefsAfter(
	ctx context.Context,
	ind QuadIndex,
	vals []uint64,
	after Int64Value,
	cb func(Int64Value, func() (bool, error)) error,
) error {
	// Require a prefix with exactly one direction left to enumerate.
	if len(vals)+1 != len(ind.Dirs) {
		return errors.New("kv: index prefix must leave exactly one direction unresolved")
	}

	// Retain one transaction for the posting scan and live-quad checks.
	tx, err := qs.db.Tx(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Close()

	// Restrict the scan to the requested posting prefix.
	pref := ind.Key(vals)
	it := tx.Scan(ctx, kvoptions.WithPrefixKV(pref))
	defer it.Close()

	// Seek past the complete cursor key without skipping keys with that prefix.
	cursor := pref
	if after != 0 {
		cursor = ind.Key(append(slices.Clone(vals), uint64(after)))
	}
	valid := kv.Seek(ctx, it, cursor)
	if after != 0 && valid && it.Key().Compare(cursor) == 0 {
		valid = it.Next(ctx)
	}

	// Decode each distinct reference and lend its live-posting check to cb.
	for ; valid; valid = it.Next(ctx) {
		// Recover the remaining node ID from the posting key.
		key := it.Key()
		if len(key) != len(pref) {
			return errors.New("kv: unexpected index key shape")
		}
		last := key[len(key)-1]
		prefixLast := pref[len(pref)-1]
		if len(last) < len(prefixLast) {
			return errors.New("kv: malformed index key")
		}
		refID, err := parseUint64IndexKey(last[len(prefixLast):])
		if err != nil {
			return err
		}

		// Inspect quad liveness only when the caller needs it.
		ids, err := decodeIndex(it.Val())
		if err != nil {
			return err
		}
		var (
			liveChecked bool
			live        bool
		)
		hasLive := func() (bool, error) {
			if liveChecked {
				return live, nil
			}
			liveChecked = true
			for _, id := range ids {
				p, err := qs.getPrimitiveFromLog(ctx, tx, id)
				if err == kv.ErrNotFound {
					continue
				}
				if err != nil {
					return false, err
				}
				if p != nil && !p.Deleted {
					live = true
					return true, nil
				}
			}
			return false, nil
		}

		// Let the caller stop after its bounded selection.
		if err := cb(Int64Value(refID), hasLive); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	return it.Err()
}

// parseUint64IndexKey decodes a complete base62 posting ID.
func parseUint64IndexKey(b []byte) (uint64, error) {
	// Require at least one base62 digit.
	if len(b) == 0 {
		return 0, io.ErrUnexpectedEOF
	}

	// Decode digits without overflowing the durable node ID.
	var out uint64
	for _, c := range b {
		d, ok := decodeBase62Digit(c)
		if !ok {
			return 0, errors.New("kv: invalid base62 index key")
		}
		if out > (^uint64(0)-uint64(d))/62 {
			return 0, errors.New("kv: out-of-range index key")
		}
		out = out*62 + uint64(d)
	}
	return out, nil
}

// decodeBase62Digit returns the numeric value of one posting-key digit.
func decodeBase62Digit(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'z':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'Z':
		return c - 'A' + 36, true
	default:
		return 0, false
	}
}
