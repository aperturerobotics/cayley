package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&Intersect{})
}

var _ linkedql.PathStep = (*Intersect)(nil)

// Intersect represents .intersect() and .and().
type Intersect struct {
	From  linkedql.PathStep   `json:"from"`
	Steps []linkedql.PathStep `json:"steps"`
}

// Description implements Step.
func (s *Intersect) Description() string {
	return "resolves to all the same values resolved by the from step and the provided steps."
}

// BuildPath implements linkedql.PathStep.
func (s *Intersect) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	p := fromPath
	for _, step := range s.Steps {
		stepPath, err := step.BuildPath(ctx, qs, ns)
		if err != nil {
			return nil, err
		}
		p = p.And(stepPath)
	}
	return p, nil
}
