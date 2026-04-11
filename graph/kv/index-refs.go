package kv

import (
	"context"
	"io"
	"math/big"

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
	if len(vals)+1 != len(ind.Dirs) {
		return errors.New("kv: index prefix must leave exactly one direction unresolved")
	}
	tx, err := qs.db.Tx(ctx, false)
	if err != nil {
		return err
	}
	defer tx.Close()

	pref := ind.Key(vals)
	it := tx.Scan(ctx, kvoptions.WithPrefixKV(pref))
	defer it.Close()

	for it.Next(ctx) {
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
		if err := cb(Int64Value(refID), hasLive); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	return nil
}

func parseUint64IndexKey(b []byte) (uint64, error) {
	if len(b) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	var i big.Int
	if _, ok := i.SetString(string(b), 62); !ok {
		return 0, errors.New("kv: invalid base62 index key")
	}
	if i.Sign() < 0 || i.BitLen() > 64 {
		return 0, errors.New("kv: out-of-range index key")
	}
	return i.Uint64(), nil
}
