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
	linkedql.Register(&Has{})
}

var _ linkedql.PathStep = (*Has)(nil)

// Has corresponds to .has().
type Has struct {
	From     linkedql.PathStep      `json:"from"`
	Property *linkedql.PropertyPath `json:"property"`
	Values   []quad.Value           `json:"values"`
}

// Description implements Step.
func (s *Has) Description() string {
	return "filters all paths which are, at this point, on the subject for the given predicate and object, but do not follow the path, merely filter the possible paths. Usually useful for starting with all nodes, or limiting to a subset depending on some predicate/value pair."
}

// BuildPath implements linkedql.PathStep.
func (s *Has) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	viaPath, err := s.Property.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Has(viaPath, linkedql.AbsoluteValues(s.Values, ns)...), nil
}
