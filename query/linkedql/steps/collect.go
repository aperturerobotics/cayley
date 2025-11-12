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
	linkedql.Register(&Collect{})
}

var _ linkedql.PathStep = (*Collect)(nil)

// Collect corresponds to .view().
type Collect struct {
	From linkedql.PathStep `json:"from"`
	Name quad.IRI          `json:"name"`
}

// Description implements Step.
func (s *Collect) Description() string {
	return "Recursively resolves values of a list (also known as RDF collection)"
}

var (
	first = quad.IRI("rdf:first").Full()
	rest  = quad.IRI("rdf:rest").Full()
)

// BuildPath implements linkedql.PathStep.
func (s *Collect) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	p := fromPath.
		Out(s.Name).
		Save(first, string(first)).
		Save(rest, string(rest)).
		Or(
			fromPath.Out(s.Name).FollowRecursive(rest, -1, nil).
				Save(first, string(first)).
				Save(rest, string(rest)),
		).
		Or(fromPath.Save(s.Name, string(s.Name)))
	return p, nil
}
