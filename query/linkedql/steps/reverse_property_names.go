package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&ReversePropertyNames{})
}

var _ linkedql.PathStep = (*ReversePropertyNames)(nil)

// ReversePropertyNames corresponds to .reversePropertyNames().
type ReversePropertyNames struct {
	From linkedql.PathStep `json:"from"`
}

// Description implements Step.
func (s *ReversePropertyNames) Description() string {
	return "gets the list of predicates that are pointing in to a node."
}

// BuildPath implements linkedql.PathStep.
func (s *ReversePropertyNames) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.InPredicates(), nil
}
