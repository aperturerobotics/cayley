package steps

import (
	"context"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/quad/voc"
	"github.com/aperturerobotics/cayley/query/linkedql"
	"github.com/aperturerobotics/cayley/query/path"
	"github.com/aperturerobotics/cayley/query/shape"
)

func init() {
	linkedql.Register(&Like{})
}

var _ linkedql.PathStep = (*Like)(nil)

// Like corresponds to like().
type Like struct {
	From    linkedql.PathStep `json:"from"`
	Pattern string            `json:"likePattern"`
}

// Description implements Operator.
func (s *Like) Description() string {
	return "Like filters out values that do not match given pattern."
}

// BuildPath implements PathStep.
func (s *Like) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Filters(shape.Wildcard{Pattern: s.Pattern}), nil
}
