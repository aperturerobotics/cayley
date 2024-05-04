package steps

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/query/path"
	"github.com/cayleygraph/cayley/quad/voc"
)

func init() {
	linkedql.Register(&Back{})
}

var _ linkedql.PathStep = (*Back)(nil)

// Back corresponds to .back().
type Back struct {
	From linkedql.PathStep `json:"from"`
	Name string            `json:"name"`
}

// Description implements Step.
func (s *Back) Description() string {
	return "resolves to the values of the previous the step or the values assigned to name in a former step."
}

// BuildPath implements linkedql.PathStep.
func (s *Back) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Back(s.Name), nil
}
