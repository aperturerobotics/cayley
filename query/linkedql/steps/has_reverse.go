package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&HasReverse{})
}

var _ linkedql.PathStep = (*HasReverse)(nil)

// HasReverse corresponds to .hasR().
type HasReverse struct {
	From     linkedql.PathStep      `json:"from"`
	Property *linkedql.PropertyPath `json:"property"`
	Values   []quad.Value           `json:"values"`
}

// Description implements Step.
func (s *HasReverse) Description() string {
	return "is the same as Has, but sets constraint in reverse direction."
}

// BuildPath implements linkedql.PathStep.
func (s *HasReverse) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	viaPath, err := s.Property.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.HasReverse(viaPath, linkedql.AbsoluteValues(s.Values, ns)...), nil
}
