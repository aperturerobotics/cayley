package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&Both{})
}

var _ linkedql.PathStep = (*Both)(nil)

// Both corresponds to .both().
type Both struct {
	From       linkedql.PathStep      `json:"from"`
	Properties *linkedql.PropertyPath `json:"properties"`
}

// Description implements Step.
func (s *Both) Description() string {
	return "is like View but resolves to both the object values and references to the values of the given properties in via. It is the equivalent for the Union of View and ViewReverse of the same property."
}

// BuildPath implements linkedql.PathStep.
func (s *Both) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	viaPath, err := s.Properties.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Both(viaPath), nil
}
