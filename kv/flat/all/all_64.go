//go:build !386 && !arm

package all

// Backends that don't support 32bit

import (
	_ "github.com/aperturerobotics/cayley/kv/flat/pebble"
)
