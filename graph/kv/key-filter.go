package kv

import (
	"hash/maphash"
	"math/bits"

	"github.com/aperturerobotics/cayley/kv"
)

// keyFilterBitsPerKey sizes a key filter for about one percent false positives.
const keyFilterBitsPerKey = 10

// keyFilterMaxBits bounds a key filter at 256 MiB. A filter holding more keys
// than it was sized for reports more false positives but stays exact about
// absent keys.
const keyFilterMaxBits = 1 << 31

// keyFilterProbes is the number of bits set and tested per key.
const keyFilterProbes = 7

// keyFilter is a Bloom filter over KV keys. A key it does not report was
// never added; a key it reports may not have been.
type keyFilter struct {
	// words holds the filter bits.
	words []uint64
	// mask selects a bit index; the bit count is a power of two.
	mask uint64
	// seed keys the hash for this process.
	seed maphash.Seed
}

// newKeyFilter returns an empty filter sized for n keys.
func newKeyFilter(n uint64) *keyFilter {
	size := min(max(n*keyFilterBitsPerKey, 64), keyFilterMaxBits)
	size = 1 << bits.Len64(size-1)
	return &keyFilter{
		words: make([]uint64, size/64),
		mask:  size - 1,
		seed:  maphash.MakeSeed(),
	}
}

// add records key.
func (f *keyFilter) add(key kv.Key) {
	h1, h2 := f.hash(key)
	for i := range uint64(keyFilterProbes) {
		bit := (h1 + i*h2) & f.mask
		f.words[bit/64] |= 1 << (bit % 64)
	}
}

// has reports whether key may have been added.
func (f *keyFilter) has(key kv.Key) bool {
	h1, h2 := f.hash(key)
	for i := range uint64(keyFilterProbes) {
		bit := (h1 + i*h2) & f.mask
		if f.words[bit/64]&(1<<(bit%64)) == 0 {
			return false
		}
	}
	return true
}

// hash returns the two hashes that derive the probe positions. Each key part
// is hashed with its length so part boundaries cannot collide.
func (f *keyFilter) hash(key kv.Key) (uint64, uint64) {
	// Hash every part under this filter's seed.
	var h maphash.Hash
	h.SetSeed(f.seed)
	for _, part := range key {
		maphash.WriteComparable(&h, len(part))
		_, _ = h.Write(part)
	}

	// Derive an odd step so the probes cover distinct bits.
	sum := h.Sum64()
	return sum, bits.RotateLeft64(sum, 32) | 1
}
