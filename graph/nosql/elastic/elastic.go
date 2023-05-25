package elastic

import (
	"context"

	"github.com/cayleygraph/cayley/graph"
	"github.com/hidal-go/hidalgo/legacy/nosql"
	"github.com/hidal-go/hidalgo/legacy/nosql/elastic"

	//import hidal-go first so the registration of the no sql stores occurs before quadstore iterates for registration
	gnosql "github.com/cayleygraph/cayley/graph/nosql"
)

const Type = elastic.Name

func Create(ctx context.Context, addr string, opt graph.Options) (nosql.Database, error) {
	return elastic.Dial(ctx, addr, gnosql.DefaultDBName, nosql.Options(opt))
}

func Open(ctx context.Context, addr string, opt graph.Options) (nosql.Database, error) {
	return elastic.Dial(ctx, addr, gnosql.DefaultDBName, nosql.Options(opt))
}
