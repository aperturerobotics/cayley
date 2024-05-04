package linkedql

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/jsonld"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query"
	"github.com/aperturerobotics/cayley/query/path"
)

var _ query.Iterator = (*ValueIterator)(nil)

// ValueIterator is an iterator of values from the graph.
type ValueIterator struct {
	Namer   refs.Namer
	path    *path.Path
	scanner iterator.Scanner
	err     error
}

// NewValueIterator returns a new ValueIterator for a path and namer.
func NewValueIterator(p *path.Path, namer refs.Namer) *ValueIterator {
	return &ValueIterator{Namer: namer, path: p}
}

// NewValueIteratorFromPathStep attempts to build a path from PathStep and return a new ValueIterator of it.
// If BuildPath fails returns error.
func NewValueIteratorFromPathStep(ctx context.Context, step PathStep, qs graph.QuadStore, ns *voc.Namespaces) (*ValueIterator, error) {
	p, err := step.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return NewValueIterator(p, qs), nil
}

// Next implements query.Iterator.
func (it *ValueIterator) Next(ctx context.Context) bool {
	if it.scanner == nil {
		it.scanner = it.path.BuildIterator(ctx).Iterate(ctx)
	}
	return it.scanner.Next(ctx)
}

// Value returns the current value
func (it *ValueIterator) Value(ctx context.Context) (quad.Value, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	if it.scanner == nil {
		return nil, nil
	}
	res, err := it.scanner.Result(ctx)
	if err != nil {
		it.err = err
		return nil, err
	}
	rname, err := it.Namer.NameOf(ctx, res)
	if err != nil {
		it.err = err
	}
	return rname, err
}

// Result implements query.Iterator.
func (it *ValueIterator) Result(ctx context.Context) (interface{}, error) {
	if err := it.Err(); err != nil {
		return nil, err
	}
	// FIXME(iddan): only convert when collation is JSON/JSON-LD, leave as Ref otherwise
	res, err := it.Value(ctx)
	if err != nil {
		return nil, err
	}
	return jsonld.FromValue(res), nil
}

// Err implements query.Iterator.
func (it *ValueIterator) Err() error {
	if it.err != nil {
		return it.err
	}
	if it.scanner == nil {
		return nil
	}
	return it.scanner.Err()
}

// Close implements query.Iterator.
func (it *ValueIterator) Close() error {
	if it.scanner == nil {
		return nil
	}
	return it.scanner.Close()
}
