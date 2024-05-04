package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&Labels{})
}

var _ linkedql.PathStep = (*Labels)(nil)

// Labels corresponds to .labels().
type Labels struct {
	From linkedql.PathStep `json:"from"`
}

// Description implements Step.
func (s *Labels) Description() string {
	return "gets the list of inbound and outbound quad labels"
}

// BuildPath implements linkedql.PathStep.
func (s *Labels) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Labels(), nil
}
