<div align="center">
  <h3>Cayley: graph storage and query for Go</h3>

  <p>
    Store linked data as quads, walk relationships from Go, and serve graph
    queries from the command line or over HTTP, on the storage backend you choose.
  </p>

  <p>
    <a href="https://godoc.org/github.com/aperturerobotics/cayley">
      <img src="https://godoc.org/github.com/aperturerobotics/cayley?status.svg" alt="GoDoc" />
    </a>
    <a href="https://deepwiki.com/aperturerobotics/cayley">
      <img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki" />
    </a>
  </p>
</div>

## Overview

**Cayley** is a Go graph database for
[Linked Data](https://www.w3.org/standards/semanticweb/data). It stores each fact
as a quad: a subject, a predicate, an object, and an optional label. Quads hold
RDF datasets, the relationships in an application, metadata indexes, and any
other facts you want to traverse as a graph.

A Go program can embed Cayley, move it between storage backends, query it from
code, and serve it over HTTP. The top-level package opens stores, adds quads,
and walks paths. The packages below it hold the storage interfaces, iterators,
query languages, quad formats, import and export, and HTTP handlers.

This is Aperture's Apache-2.0 fork of the
[upstream Cayley project](https://github.com/cayleygraph/cayley).

## Features

### Graph Model

- Quads with subject, predicate, object, and optional label fields
- Typed quad values for IRIs, blank nodes, strings, numbers, booleans, and time
- Indexes over quad directions for fast traversal
- Transactions and batch writers for atomic updates
- A path API for graph walks written in Go

### Storage Backends

The graph layer runs on any of these storage engines:

- **In-memory store** for tests, examples, and short-lived graphs
- **Key-value stores** through the `graph/kv` adapter:
  - [BoltDB/bbolt](https://github.com/etcd-io/bbolt)
  - [Badger](https://github.com/dgraph-io/badger)
  - an in-memory B-tree
- **SQL stores**:
  - [CockroachDB](https://github.com/cockroachdb/cockroach)
  - [PostgreSQL](https://www.postgresql.org/)
  - [MySQL](https://github.com/go-sql-driver/mysql)
  - [SQLite](https://www.sqlite.org/)

### Query and Data Formats

Cayley runs as a Go library, a command-line tool, or an HTTP service:

- The Go path API in `github.com/aperturerobotics/cayley/query/path`
- Query languages: Gizmo, GraphQL, MQL, and S-expressions
- Commands to init, load, dump, upgrade, query, convert, and serve a graph
- Quad formats: N-Quads, JSON-LD, GraphML, GML, DOT, and packed quads

## Getting Started

Install the CLI:

```bash
go install github.com/aperturerobotics/cayley/cmd/cayley@latest
```

Create an in-memory graph and query it from Go:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aperturerobotics/cayley"
	"github.com/aperturerobotics/cayley/quad"
)

func main() {
	ctx := context.Background()
	store, err := cayley.NewMemoryGraph(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if err := store.AddQuad(ctx, cayley.Quad("alice", "knows", "bob", nil)); err != nil {
		log.Fatal(err)
	}

	path := cayley.StartPath(store, quad.String("alice")).Out(quad.String("knows"))
	err = path.Iterate(nil).EachValue(ctx, nil, func(value quad.Value) error {
		fmt.Println(quad.NativeOf(value))
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

Run a query through the CLI:

```bash
cayley query --db memstore --load ./data/testdata.nq --load_format nquads --lang gizmo 'g.V().Limit(1).All()'
```

Load data into a persistent store:

```bash
cayley init --db bolt --dbpath ./graph.db
cayley load --db bolt --dbpath ./graph.db --load ./data/testdata.nq --load_format nquads
cayley repl --db bolt --dbpath ./graph.db --lang gizmo
```

Serve HTTP on the local interface:

```bash
cayley http --db bolt --dbpath ./graph.db --host 127.0.0.1:64210
```

## Path API

The path API in `github.com/aperturerobotics/cayley/query/path`, re-exported
as `cayley.Path`, walks a graph from Go. A path starts at one or more nodes and
chains traversal steps. Nothing runs until you build an iterator, so callers can
compose, pass around, and extend a path before it executes.

### Concepts

A **path** is the builder you chain steps onto. Each step, such as `Out`, `Has`
or `Tag`, returns a new `*Path`, so you can branch and reuse a path freely.
`StartPath` binds a path to a `QuadStore` so it can execute. A path with no
store is a **morphism**: a step sequence that `Follow`, `Has`, or
`BuildIteratorOn` later applies to a store. `StartMorphism` builds one, so many
callers can share one traversal.

A **shape** is the query tree a path compiles to, returned by `p.Shape()`. It
describes what to match on any backend. Its `Optimize` pass lets the store push
work into its indexes before execution. The path builder produces shapes, so
most code never handles them.

An **iterator** scans a store for the matches of a shape, and
`p.BuildIterator(ctx)` returns one. It yields `graph.Ref` handles, and
`store.NameOf` resolves each to its `quad.Value`. `p.Iterate(ctx)` wraps the
iterator with `EachValue`, `TagValues`, `AllValues`, and `Count`, so most code
never drives it by hand.

Build a path with `cayley.StartPath`, then chain steps and iterate:

```go
store, _ := cayley.NewMemoryGraph(ctx)
store.AddQuad(ctx, cayley.Quad("alice", "knows", "bob", nil))
store.AddQuad(ctx, cayley.Quad("bob", "knows", "carol", nil))

// Who does alice know?
p := cayley.StartPath(store, quad.String("alice")).Out(quad.String("knows"))
p.Iterate(ctx).EachValue(ctx, nil, func(v quad.Value) error {
	fmt.Println(quad.NativeOf(v)) // bob
	return nil
})
```

`Out` follows a predicate forward (subject to object); `In` follows it backward
(object to subject). Reverse a lookup by swapping the direction:

```go
// Who knows bob? Traverse the "knows" edge backward.
p := cayley.StartPath(store, quad.String("bob")).In(quad.String("knows"))
// yields: alice
```

`Has` keeps only the nodes with a matching outbound edge, which filters a set
down to nodes of a given type or property. This example walks inbound links
from a keypair, then keeps the nodes whose type is one of the wanted values.

```go
// Keep only nodes that link to one of the wanted type values.
p := cayley.StartPath(store, quad.String("keypair-1")).
	In(quad.String("object-to-keypair")).
	Has(quad.String("type"), quad.String("cluster"), quad.String("task"))
```

`Tag` saves the node at a step under a name, so one iteration returns several
values at once. `TagValues` passes them to your callback as a map:

```go
p := cayley.StartPath(store, quad.String("alice")).
	Tag("person").
	Out(quad.String("knows")).
	Tag("friend")
p.Iterate(ctx).TagValues(ctx, nil, func(tags map[string]quad.Value) error {
	fmt.Println(quad.NativeOf(tags["person"]), "knows", quad.NativeOf(tags["friend"]))
	return nil
})
```

`FollowRecursive` follows one predicate repeatedly to reach every node at the
end of a chain of edges. An optional depth tag records how many hops each result
took:

```go
store.AddQuad(ctx, cayley.Quad("a", "ref", "b", nil))
store.AddQuad(ctx, cayley.Quad("b", "ref", "c", nil))

// All nodes reachable from "a" through "ref" edges: b, c
p := cayley.StartPath(store, quad.String("a")).
	FollowRecursive(quad.String("ref"), -1, []string{"depth"})
```

`LabelContext` limits the steps after it to quads with a given label, so one
predicate can mean different things in different subgraphs. To drive iteration
yourself, call `p.BuildIterator(ctx)` and resolve each result with
`store.NameOf`.

### More operations

Traversal directions:

- `Both(pred)` follows `pred` in both directions and yields every neighbor.
- `Reverse()` mirrors a whole path, turning a forward walk into a backward one.
- `InPredicates()` and `OutPredicates()` yield the predicates on a node, so you
  can see which edges exist before following them.

Set operations take another path and join it with this one:

- `And(other)` keeps the nodes both paths reach, `Or(other)` keeps the nodes
  either reaches, and `Except(other)` removes the other path's nodes.
- `Has(pred, nodes...)` keeps nodes with an outbound `pred` edge to one of
  `nodes`. `HasReverse(pred, nodes...)` keeps nodes with an inbound `pred` edge
  from one of them. `HasPath(sub)` keeps nodes from which `sub` matches. These
  filter the current set and leave the path on the same nodes.
- `Follow(morphism)` applies a morphism, built once with `StartMorphism`, to the
  current nodes.

Saving values along the walk:

- `Save(pred, tag)` saves the node one hop across `pred` under `tag` and leaves
  the path where it is, so one iteration returns several related fields.
  `SaveReverse`, `SaveOptional`, and `SaveOptionalReverse` follow the edge
  inbound or allow it to be missing.
- `Tag(name)` saves the current node. `Back(name)` returns to an earlier tagged
  node and keeps the constraints added after it, so you can filter a node by a
  property further along the walk and still return the node.

Filtering and shaping results:

- `Is(nodes...)` keeps only the given nodes. `Filter` and `Filters` compare
  values with an `iterator.Operator` such as `<` or `>=`, and `Regex` matches
  strings against a pattern.
- `Unique()` removes duplicates, `Skip(n)` and `Limit(n)` page the results, and
  `Count()` replaces the results with their count.

This example combines several of them:

```go
// Ancestors of "alice" through "parent" who are tagged as "person",
// returning the ancestor node plus its saved "name".
p := cayley.StartPath(store, quad.String("alice")).
	FollowRecursive(quad.String("parent"), -1, nil).
	Has(quad.String("type"), quad.String("person")).
	Save(quad.String("name"), "name").
	Unique()
```

### Reference

The [package documentation](https://pkg.go.dev/github.com/aperturerobotics/cayley/query/path)
describes every step.

## Development

`make` runs the build and development commands through the
[Aperture build tool](https://github.com/aperturerobotics/common):

| Command          | Description                                 |
| ---------------- | ------------------------------------------- |
| `make gen`       | Generate protobuf code                      |
| `make test`      | Run tests                                   |
| `make lint`      | Run golangci-lint                           |
| `make fix`       | Run golangci-lint with `--fix`              |
| `make format`    | Format Go code                              |
| `make goimports` | Run goimports                               |
| `make deps`      | Ensure build dependencies are installed     |
| `make vendor`    | Update the vendor directory                 |
| `make outdated`  | Show outdated dependencies                  |
| `make clean`     | Remove generated files and cache            |
| `make release`   | Run goreleaser                              |

Where to start reading:

- [`cayley.go`](./cayley.go): top-level Go API
- [`graph/`](./graph): quad store, writer, transaction, iterator, and backend APIs
- [`query/`](./query): query languages and the path API
- [`quad/`](./quad): quad value model and import/export formats
- [`cmd/cayley/`](./cmd/cayley): the command-line tool and HTTP server
- [`examples/`](./examples): small embedded graph examples

## License

Cayley is licensed under [Apache-2.0](./LICENSE).
