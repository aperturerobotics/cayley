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
	linkedql.Register(&Vertex{})
}

var _ linkedql.PathStep = (*Vertex)(nil)

// Vertex corresponds to g.Vertex() and g.V().
type Vertex struct {
	Values []quad.Value `json:"values"`
}

// Description implements Step.
func (s *Vertex) Description() string {
	return "resolves to all the existing objects and primitive values in the graph. If provided with values resolves to a sublist of all the existing values in the graph."
}

// BuildPath implements linkedql.PathStep.
func (s *Vertex) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	return path.StartPath(qs, linkedql.AbsoluteValues(s.Values, ns)...), nil
}
