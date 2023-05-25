package steps

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/query/path"
	"github.com/cayleygraph/quad/voc"
)

func init() {
	linkedql.Register(&Visit{})
}

var _ linkedql.PathStep = (*Visit)(nil)

// Visit corresponds to .view().
type Visit struct {
	From       linkedql.PathStep      `json:"from"`
	Properties *linkedql.PropertyPath `json:"properties"`
}

// Description implements Step.
func (s *Visit) Description() string {
	return "resolves to the values of the given property or properties in via of the current objects. If via is a path it's resolved values will be used as properties."
}

// BuildPath implements linkedql.PathStep.
func (s *Visit) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	viaPath, err := s.Properties.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	return fromPath.Out(viaPath), nil
}
