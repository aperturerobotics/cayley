package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/iterator"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
)

func init() {
	linkedql.Register(&GreaterThanEquals{})
}

var _ linkedql.PathStep = (*GreaterThanEquals)(nil)

// GreaterThanEquals corresponds to gte().
type GreaterThanEquals struct {
	From  linkedql.PathStep `json:"from"`
	Value quad.Value        `json:"value"`
}

// Description implements Step.
func (s *GreaterThanEquals) Description() string {
	return "Greater than equals filters out values that are not greater than or equal given value"
}

// BuildPath implements linkedql.PathStep.
func (s *GreaterThanEquals) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Filter(iterator.CompareGTE, linkedql.AbsoluteValue(s.Value, ns)), nil
}
