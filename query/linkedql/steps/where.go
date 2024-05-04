package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&Where{})
}

var _ linkedql.PathStep = (*Where)(nil)

// Where corresponds to .where().
type Where struct {
	From      linkedql.PathStep `json:"from"`
	Condition linkedql.PathStep `json:"condition"`
}

// Description implements Step.
func (s *Where) Description() string {
	return "filters results that fulfill a specified condition"
}

// BuildPath implements linkedql.PathStep.
func (s *Where) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	stepPath, err := s.Condition.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.And(stepPath.Reverse()), nil
}
