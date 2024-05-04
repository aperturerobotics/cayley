package cayley

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/iterator"
	_ "github.com/cayleygraph/cayley/graph/memstore"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/path"
	_ "github.com/cayleygraph/cayley/writer"
)

var (
	StartMorphism = path.StartMorphism
	StartPath     = path.StartPath

	NewTransaction = graph.NewTransaction
)

type (
	Iterator   = iterator.Shape
	QuadStore  = graph.QuadStore
	QuadWriter = graph.QuadWriter
)

type Path = path.Path

type Handle struct {
	graph.QuadStore
	graph.QuadWriter
}

func (h *Handle) Close() error {
	err := h.QuadWriter.Close()
	h.QuadStore.Close()
	return err
}

func Triple(subject, predicate, object interface{}) quad.Quad {
	return Quad(subject, predicate, object, nil)
}

func Quad(subject, predicate, object, label interface{}) quad.Quad {
	return quad.Make(subject, predicate, object, label)
}

func NewGraph(ctx context.Context, name, dbpath string, opts graph.Options) (*Handle, error) {
	qs, err := graph.NewQuadStore(ctx, name, dbpath, opts)
	if err != nil {
		return nil, err
	}
	qw, err := graph.NewQuadWriter("single", qs, nil)
	if err != nil {
		return nil, err
	}
	return &Handle{qs, qw}, nil
}

func NewMemoryGraph(ctx context.Context) (*Handle, error) {
	return NewGraph(ctx, "memstore", "", nil)
}
