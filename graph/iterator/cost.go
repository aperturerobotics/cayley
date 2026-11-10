package iterator

import "math"

// SatAdd returns a+b for nonnegative costs and sizes, saturating at
// math.MaxInt64 instead of wrapping negative. A store may report a size near
// the int64 range for a scan it cannot count, and the planner sums and
// multiplies such sizes, so every cost combination goes through SatAdd and
// SatMul.
func SatAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

// SatMul returns a*b for nonnegative costs and sizes, saturating at
// math.MaxInt64 instead of wrapping.
func SatMul(a, b int64) int64 {
	if a > 0 && b > 0 && a > math.MaxInt64/b {
		return math.MaxInt64
	}
	return a * b
}

// containsPerNext is the cost of checking each value of a scan of rootSize
// values against an index of size: one Contains, plus one per value the index
// holds fewer than the scan.
func containsPerNext(containsCost, rootSize, size int64) int64 {
	return SatMul(containsCost, SatAdd(1, rootSize/SatAdd(size, 1)))
}
