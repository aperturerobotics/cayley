package kv

import (
	"context"

	"github.com/aperturerobotics/cayley/graph/proto"
	"github.com/aperturerobotics/cayley/kv"
	"github.com/aperturerobotics/cayley/kv/options"
)

// WalkLog calls fn with each live primitive in the log of the graph in db, in
// one read transaction. The log is keyed by the ID's decimal text, so the
// primitives come in that text's order, not by number: a caller that needs ID
// order sorts them. Deleted entries are skipped. fn must not keep p after it
// returns, and its error stops the walk.
//
// The walk reads each log entry once, in key order, and holds one primitive at
// a time, so its memory does not grow with the graph.
func WalkLog(ctx context.Context, db kv.KV, fn func(p *proto.Primitive) error) error {
	return kv.View(ctx, db, func(tx kv.Tx) error {
		// Scan the log bucket and decode each entry into one reused message.
		it := tx.Scan(ctx, options.WithPrefixKV(logIndex))
		defer it.Close()
		p := &proto.Primitive{}
		for it.Next(ctx) {
			p.Reset()
			if err := p.UnmarshalVT(it.Val()); err != nil {
				return err
			}
			if p.Deleted {
				continue
			}
			if err := fn(p); err != nil {
				return err
			}
		}
		return it.Err()
	})
}
