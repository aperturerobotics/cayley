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
	linkedql.Register(&LessThanEquals{})
}

var _ linkedql.PathStep = (*LessThanEquals)(nil)

// LessThanEquals corresponds to lte().
type LessThanEquals struct {
	From  linkedql.PathStep `json:"from"`
	Value quad.Value        `json:"value"`
}

// Description implements Step.
func (s *LessThanEquals) Description() string {
	return "Less than equals filters out values that are not less than or equal given value"
}

// BuildPath implements linkedql.PathStep.
func (s *LessThanEquals) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Filter(iterator.CompareLTE, linkedql.AbsoluteValue(s.Value, ns)), nil
}
