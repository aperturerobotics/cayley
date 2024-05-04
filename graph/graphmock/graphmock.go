package graphmock

import (
	"context"
	"strconv"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

var (
	_ graph.Ref = IntVal(0)
	_ graph.Ref = StringNode("")
)

type IntVal int

func (v IntVal) Key() interface{} { return v }

type StringNode string

func (s StringNode) Key() interface{} { return s }

// Oldstore is a mocked version of the QuadStore interface, for use in tests.
type Oldstore struct {
	Parse bool
	Data  []string
	Iter  iterator.Shape
}

func (qs *Oldstore) valueAt(i int) quad.Value {
	if !qs.Parse {
		return quad.Raw(qs.Data[i])
	}
	iv, err := strconv.Atoi(qs.Data[i])
	if err == nil {
		return quad.Int(iv)
	}
	return quad.String(qs.Data[i])
}

func (qs *Oldstore) ValueOf(ctx context.Context, s quad.Value) (graph.Ref, error) {
	if s == nil {
		return nil, nil
	}
	for i := range qs.Data {
		if va := qs.valueAt(i); va != nil && s.String() == va.String() {
			return iterator.Int64Node(i), nil
		}
	}
	return nil, nil
}

func (qs *Oldstore) NewQuadWriter(ctx context.Context) (quad.WriteCloser, error) {
	return nopWriter{}, nil
}

type nopWriter struct{}

func (nopWriter) WriteQuad(ctx context.Context, q quad.Quad) error {
	return nil
}

func (nopWriter) WriteQuads(ctx context.Context, buf []quad.Quad) (int, error) {
	return len(buf), nil
}

func (nopWriter) Close() error {
	return nil
}

func (qs *Oldstore) ApplyDeltas(context.Context, []graph.Delta, graph.IgnoreOpts) error { return nil }

func (qs *Oldstore) Quad(context.Context, graph.Ref) quad.Quad { return quad.Quad{} }

func (qs *Oldstore) QuadIterator(ctx context.Context, d quad.Direction, i graph.Ref) iterator.Shape {
	return qs.Iter
}

func (qs *Oldstore) QuadIteratorSize(ctx context.Context, d quad.Direction, val graph.Ref) (refs.Size, error) {
	st, err := qs.Iter.Stats(ctx)
	return st.Size, err
}

func (qs *Oldstore) NodesAllIterator(ctx context.Context) iterator.Shape { return &iterator.Null{} }

func (qs *Oldstore) QuadsAllIterator(ctx context.Context) iterator.Shape { return &iterator.Null{} }

func (qs *Oldstore) NameOf(ctx context.Context, v graph.Ref) (quad.Value, error) {
	switch v := v.(type) {
	case iterator.Int64Node:
		i := int(v)
		if i < 0 || i >= len(qs.Data) {
			return nil, nil
		}
		return qs.valueAt(i), nil
	case StringNode:
		if qs.Parse {
			return quad.String(v), nil
		}
		return quad.Raw(string(v)), nil
	default:
		return nil, nil
	}
}

func (qs *Oldstore) Size() int64 { return 0 }

func (qs *Oldstore) DebugPrint() {}

func (qs *Oldstore) OptimizeIterator(ctx context.Context, it iterator.Shape) (iterator.Shape, bool, error) {
	return iterator.NewNull(), false, nil
}

func (qs *Oldstore) Close() error { return nil }

func (qs *Oldstore) QuadDirection(_ context.Context, _ graph.Ref, _ quad.Direction) (graph.Ref, error) {
	return iterator.Int64Node(0), nil
}

func (qs *Oldstore) RemoveQuad(t quad.Quad) {}

func (qs *Oldstore) Type() string { return "oldmockstore" }

type Store struct {
	Data []quad.Quad
}

var _ graph.QuadStore = &Store{}

func (qs *Store) ValueOf(ctx context.Context, s quad.Value) (graph.Ref, error) {
	for _, q := range qs.Data {
		if q.Subject == s || q.Object == s {
			return refs.PreFetched(s), nil
		}
	}
	return nil, nil
}

func (qs *Store) ApplyDeltas(context.Context, []graph.Delta, graph.IgnoreOpts) error { return nil }

func (qs *Store) NewQuadWriter(ctx context.Context) (quad.WriteCloser, error) {
	return nopWriter{}, nil
}

type quadValue struct {
	q quad.Quad
}

func (q quadValue) Key() interface{} {
	return q.q.String()
}

func (qs *Store) Quad(ctx context.Context, v graph.Ref) (quad.Quad, error) {
	return v.(quadValue).q, nil
}

func (qs *Store) NameOf(ctx context.Context, v graph.Ref) (quad.Value, error) {
	if v == nil {
		return nil, nil
	}
	return v.(refs.PreFetchedValue).NameOf(), nil
}

func (qs *Store) RemoveQuad(t quad.Quad) {}

func (qs *Store) Type() string { return "mockstore" }

func (qs *Store) QuadDirection(ctx context.Context, v graph.Ref, d quad.Direction) (graph.Ref, error) {
	qv, err := qs.Quad(ctx, v)
	if err != nil {
		return nil, err
	}
	return refs.PreFetched(qv.Get(d)), nil
}

func (qs *Store) Close() error { return nil }

func (qs *Store) DebugPrint() {}

func (qs *Store) QuadIterator(ctx context.Context, d quad.Direction, i graph.Ref) iterator.Shape {
	fixed := iterator.NewFixed()
	v := i.(refs.PreFetchedValue).NameOf()
	for _, q := range qs.Data {
		if q.Get(d) == v {
			fixed.Add(quadValue{q})
		}
	}
	return fixed
}

func (qs *Store) QuadIteratorSize(ctx context.Context, d quad.Direction, val graph.Ref) (refs.Size, error) {
	v := val.(refs.PreFetchedValue).NameOf()
	sz := refs.Size{Exact: true}
	for _, q := range qs.Data {
		if q.Get(d) == v {
			sz.Value++
		}
	}
	return sz, nil
}

func (qs *Store) NodesAllIterator(ctx context.Context) iterator.Shape {
	set := make(map[string]bool)
	for _, q := range qs.Data {
		for _, d := range quad.Directions {
			n, err := qs.NameOf(ctx, refs.PreFetched(q.Get(d)))
			if err != nil {
				return iterator.NewError(err)
			}
			if n != nil {
				set[n.String()] = true
			}
		}
	}
	fixed := iterator.NewFixed()
	for k := range set {
		fixed.Add(refs.PreFetched(quad.Raw(k)))
	}
	return fixed
}

func (qs *Store) QuadsAllIterator(ctx context.Context) iterator.Shape {
	fixed := iterator.NewFixed()
	for _, q := range qs.Data {
		fixed.Add(quadValue{q})
	}
	return fixed
}

func (qs *Store) Stats(ctx context.Context, exact bool) (graph.Stats, error) {
	set := make(map[string]struct{})
	for _, q := range qs.Data {
		for _, d := range quad.Directions {
			n, err := qs.NameOf(ctx, refs.PreFetched(q.Get(d)))
			if err != nil {
				return graph.Stats{}, err
			}
			if n != nil {
				set[n.String()] = struct{}{}
			}
		}
	}
	return graph.Stats{
		Nodes: refs.Size{Value: int64(len(set)), Exact: true},
		Quads: refs.Size{Value: int64(len(qs.Data)), Exact: true},
	}, nil
}
