package steps

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/quad/voc"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/query/path"
)

func init() {
	linkedql.Register(&Optional{})
}

var _ linkedql.PathStep = (*Optional)(nil)

// Optional corresponds to .optional().
type Optional struct {
	From linkedql.PathStep `json:"from"`
	Step linkedql.PathStep `json:"step"`
}

// Description implements Step.
func (s *Optional) Description() string {
	return "attempts to follow the given path from the current entity / value, if fails the entity / value will still be kept in the results"
}

// BuildPath implements linkedql.PathStep.
func (s *Optional) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	p, err := s.Step.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Optional(p), nil
}
