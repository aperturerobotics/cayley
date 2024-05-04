package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&VisitReverse{})
}

var _ linkedql.PathStep = (*VisitReverse)(nil)

// VisitReverse corresponds to .viewReverse().
type VisitReverse struct {
	From       linkedql.PathStep      `json:"from"`
	Properties *linkedql.PropertyPath `json:"properties"`
}

// Description implements Step.
func (s *VisitReverse) Description() string {
	return "is the inverse of View. Starting with the nodes in `path` on the object, follow the quads with predicates defined by `predicatePath` to their subjects."
}

// BuildPath implements linkedql.PathStep.
func (s *VisitReverse) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	viaPath, err := s.Properties.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.In(viaPath), nil
}
