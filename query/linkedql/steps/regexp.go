package steps

import (
	"context"
	"regexp"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/query/linkedql"
	"github.com/cayleygraph/cayley/query/path"
	"github.com/cayleygraph/cayley/quad/voc"
)

func init() {
	linkedql.Register(&RegExp{})
}

var _ linkedql.PathStep = (*RegExp)(nil)

// RegExp corresponds to regex().
type RegExp struct {
	From        linkedql.PathStep `json:"from"`
	Expression  string            `json:"expression"`
	IncludeIRIs bool              `json:"includeIRIs,omitempty"`
}

// Description implements Step.
func (s *RegExp) Description() string {
	return "RegExp filters out values that do not match given pattern. If includeIRIs is set to true it matches IRIs in addition to literals."
}

// BuildPath implements PathStep.
func (s *RegExp) BuildPath(ctx context.Context, qs graph.QuadStore, ns *voc.Namespaces) (*path.Path, error) {
	fromPath, err := s.From.BuildPath(ctx, qs, ns)
	if err != nil {
		return nil, err
	}
	pattern, err := regexp.Compile(s.Expression)
	if err != nil {
		return nil, err
	}
	if s.IncludeIRIs {
		return fromPath.RegexWithRefs(pattern), nil
	}
	return fromPath.RegexWithRefs(pattern), nil
}
