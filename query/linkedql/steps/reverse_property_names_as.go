package steps

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/quad/voc"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/query/path"
)

func init() {
	linkedql.Register(&ReversePropertyNamesAs{})
}

var _ linkedql.PathStep = (*ReversePropertyNamesAs)(nil)

// ReversePropertyNamesAs corresponds to .reversePropertyNamesAs().
type ReversePropertyNamesAs struct {
	From linkedql.PathStep `json:"from"`
	Tag  string            `json:"tag"`
}

// Description implements Step.
func (s *ReversePropertyNamesAs) Description() string {
	return "tags the list of predicates that are pointing in to a node."
}

// BuildPath implements linkedql.PathStep.
func (s *ReversePropertyNamesAs) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.SavePredicates(true, s.Tag), nil
}
