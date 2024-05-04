package steps

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/query"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/quad/voc"
)

func init() {
	linkedql.Register(&Select{})
	linkedql.Register(&Documents{})
}

var _ linkedql.IteratorStep = (*Select)(nil)

// Select corresponds to .select().
type Select struct {
	Properties []string          `json:"properties"`
	From       linkedql.PathStep `json:"from"`
	ExcludeID  bool              `json:"excludeID"`
}

// Description implements Step.
func (s *Select) Description() string {
	return "Select returns flat records of tags matched in the query"
}

// BuildIterator implements IteratorStep
func (s *Select) BuildIterator(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (query.Iterator, error) {
	valueIt, err := linkedql.NewValueIteratorFromPathStep(ctx, s.From, qs, ns)
	if err != nil {
		return nil, err
	}
	it := linkedql.NewTagsIterator(valueIt, s.Properties, s.ExcludeID)
	return &it, nil
}

var _ linkedql.IteratorStep = (*Documents)(nil)

// Documents corresponds to .documents().
type Documents struct {
	From linkedql.PathStep `json:"from"`
}

// Description implements Step.
func (s *Documents) Description() string {
	return "Documents return documents of the tags matched in the query associated with their entity"
}

// BuildIterator implements IteratorStep
func (s *Documents) BuildIterator(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (query.Iterator, error) {
	p, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	it, err := linkedql.NewValueIterator(p, qs), nil
	if err != nil {
		return nil, err
	}
	return linkedql.NewDocumentIterator(it), nil
}
