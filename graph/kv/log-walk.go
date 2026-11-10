package kv

import (
	"bytes"
	"context"
	"strconv"

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

// LogPrefix returns the key every log entry's key extends by one part, the
// entry's ID in decimal text. Between two versions of a store, the changed
// keys under it are the primitives added and removed: the store deletes a
// quad's or node's entry with it and never reuses an ID.
func LogPrefix() kv.Key {
	return logIndex.Clone()
}

// LogKeyID returns the ID the log entry key k names, or false when k is not a
// log entry key.
func LogKeyID(k kv.Key) (uint64, bool) {
	// Match the log part, then parse the ID part.
	if len(k) != 2 || !bytes.Equal(k[0], logIndex[0]) {
		return 0, false
	}
	id, err := strconv.ParseUint(string(k[1]), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

// ReadLog returns the log entries of ids in one batch read of tx, nil for an
// ID whose entry is missing or marked deleted.
func ReadLog(ctx context.Context, tx kv.Tx, ids []uint64) ([]*proto.Primitive, error) {
	// Read every entry at once.
	keys := make([]kv.Key, len(ids))
	for i, id := range ids {
		keys[i] = logIndex.AppendBytes(uint64KeyBytesBase10(id))
	}
	vals, err := tx.GetBatch(ctx, keys)
	if err != nil {
		return nil, err
	}

	// Decode the live entries.
	out := make([]*proto.Primitive, len(ids))
	for i, v := range vals {
		if v == nil {
			continue
		}
		p := &proto.Primitive{}
		if err := p.UnmarshalVT(v); err != nil {
			return nil, err
		}
		if !p.Deleted {
			out[i] = p
		}
	}
	return out, nil
}
