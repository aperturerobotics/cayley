package kv

import (
	"bytes"
	"context"
	"slices"

	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/pkg/errors"
)

// postingScanIndex returns the index whose postings a lookup that no index
// prefix serves scans, such as one by predicate alone, or false when no index
// holds any direction in values. It prefers the index whose leading directions
// values pins most, whose prefix holds only matching postings, and then the
// index with the fewest directions, whose keys group the most links each.
func (qs *QuadStore) postingScanIndex(values map[quad.Direction]uint64) (QuadIndex, bool) {
	// Pick the longest pinned prefix, then the narrowest index holding a
	// filtered direction.
	qs.indexes.RLock()
	all := qs.indexes.all
	qs.indexes.RUnlock()
	var (
		best       QuadIndex
		bestPrefix int
		found      bool
	)
	for _, ind := range all {
		if !slices.ContainsFunc(ind.Dirs, func(d quad.Direction) bool {
			_, ok := values[d]
			return ok
		}) {
			continue
		}
		prefix := len(pinnedPrefix(ind, values))
		if !found || prefix > bestPrefix || (prefix == bestPrefix && len(ind.Dirs) < len(best.Dirs)) {
			best, bestPrefix, found = ind, prefix, true
		}
	}
	return best, found
}

// pinnedPrefix returns the values of ind's leading directions that values
// pins, stopping at the first it does not.
func pinnedPrefix(ind QuadIndex, values map[quad.Direction]uint64) []uint64 {
	var prefix []uint64
	for _, d := range ind.Dirs {
		v, ok := values[d]
		if !ok {
			break
		}
		prefix = append(prefix, v)
	}
	return prefix
}

// postingLinkIDs returns, in ascending order, the IDs whose posting keys in
// ind match values in every direction ind holds. It reads each posting of ind
// once and no primitive, so it costs one pass over the live links rather than
// a read of every ID the log ever allocated. The IDs keep the log's order, so
// a limited lookup returns what the log scan returned. The caller matches the
// directions ind does not hold, and skips primitives the log no longer holds.
func (qs *QuadStore) postingLinkIDs(ctx context.Context, tx kv.Tx, ind QuadIndex, values map[quad.Direction]uint64) ([]uint64, error) {
	// Encode each value ind holds the way its keys spell it.
	want := make([][]byte, len(ind.Dirs))
	for i, d := range ind.Dirs {
		if v, ok := values[d]; ok {
			want[i] = uint64KeyBytes(v)
		}
	}

	// Gather the postings whose key parts match, reading only the pinned
	// prefix when the lookup pins ind's leading directions.
	scan := ind.bucket()
	if prefix := pinnedPrefix(ind, values); len(prefix) != 0 {
		scan = ind.Key(prefix)
	}
	it := tx.Scan(ctx, options.WithPrefixKV(scan))
	defer it.Close()
	var ids []uint64
	for it.Next(ctx) {
		key := it.Key()
		if len(key) != 2 {
			return nil, errors.New("kv: unexpected index key shape")
		}
		if !postingKeyMatches(key[1], want) {
			continue
		}
		posted, err := decodeIndex(it.Val())
		if err != nil {
			return nil, err
		}
		ids = append(ids, posted...)
	}
	if err := it.Err(); err != nil {
		return nil, err
	}

	// Order them as the log does.
	slices.Sort(ids)
	return slices.Compact(ids), nil
}

// postingKeyMatches reports whether each part of a posting key equals the
// wanted part at its position, where one is wanted. Parts are base62, so the
// separator never occurs inside one.
func postingKeyMatches(key []byte, want [][]byte) bool {
	for _, part := range want {
		next, rest, _ := bytes.Cut(key, []byte{':'})
		key = rest
		if part != nil && !bytes.Equal(next, part) {
			return false
		}
	}
	return true
}
