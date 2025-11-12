//go:build cgo

package all

import (
	// backends requiring cgo
	_ "github.com/aperturerobotics/cayley/graph/sql/sqlite"
)
