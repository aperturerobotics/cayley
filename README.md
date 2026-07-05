<div align="center">
  <h3>Cayley: graph storage and query for Go</h3>

  <p>
    Store linked data as quads, traverse relationships from Go, and serve graph
    queries through CLI or HTTP endpoints without committing to one storage
    backend.
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

**Cayley** is a Go graph database and query layer for
[Linked Data](https://www.w3.org/standards/semanticweb/data). It stores facts as
quads: subject, predicate, object, and optional label. That shape can represent
RDF-style datasets, application relationship graphs, metadata indexes, and other
small facts that need graph traversal instead of only key lookup.

Use Cayley when your Go program needs a graph store it can embed, swap across
backends, query from code, or expose through operational tools. The core package
provides a small handle API for opening stores, adding quads, and walking paths.
The lower-level packages expose storage interfaces, iterators, query sessions,
quad formats, import/export flows, and HTTP handlers.

This is Aperture's Apache-2.0 fork of the
[upstream Cayley project](https://github.com/cayleygraph/cayley).

## Current Surface

### Graph Model

- Quads with subject, predicate, object, and optional label fields
- Typed quad values for IRIs, blank nodes, strings, numbers, booleans, and time
- Directional indexes over quad fields for traversal-heavy workloads
- Transactions and batch writers for atomic graph updates
- Path traversal API for expressing graph walks directly from Go

### Storage Backends

Cayley separates the graph layer from the storage engine. Available backends
include:

- **In-memory store** for tests, examples, and ephemeral graphs
- **Key-value stores** through the graph/KV adapter:
  - [BoltDB/bbolt](https://github.com/etcd-io/bbolt)
  - [Badger](https://github.com/dgraph-io/badger)
  - in-memory B-tree through the flat KV path
- **SQL stores**:
  - [CockroachDB](https://github.com/cockroachdb/cockroach)
  - [PostgreSQL](https://www.postgresql.org/)
  - [MySQL](https://github.com/go-sql-driver/mysql)
  - [SQLite](https://www.sqlite.org/)

### Query and Data Formats

Cayley can be used as a library, command-line tool, or HTTP service:

- Go path API through `github.com/aperturerobotics/cayley/query/path`
- Query sessions for Gizmo, GraphQL, MQL, S-expression queries, and package-level
  extension points
- CLI commands for init, load, dump, upgrade, query, REPL, HTTP serving, format
  conversion, deduplication, health checks, and schema work
- Quad readers and writers for common graph formats, including N-Quads, JSON-LD,
  GraphML, GML, DOT, and packed quads

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

The Go path API in `github.com/aperturerobotics/cayley/query/path` (re-exported
as `cayley.Path`) is the main way to walk an application graph directly from Go.
A path starts at one or more nodes and chains traversal steps; nothing runs
until you build an iterator, so a path can be composed, passed around, and
extended by callers before it executes.

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

`Has` keeps only nodes that have a matching outbound edge, which is how you
filter a set down to nodes of a given type or property. This is the shape
Spacewave uses to list graph objects of a known type reachable from a keypair:
walk the inbound links, then keep the nodes tagged with a recognized type
predicate.

```go
// Keep only nodes that link to one of the wanted type values.
p := cayley.StartPath(store, quad.String("keypair-1")).
	In(quad.String("object-to-keypair")).
	Has(quad.String("type"), quad.String("cluster"), quad.String("task"))
```

`Tag` records the node at a step under a name so a single iteration can return
several bound values at once; read them from the result map instead of
`EachValue`:

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

`FollowRecursive` walks one predicate transitively to reach every node
reachable through a chain of edges, with an optional depth tag reporting how
many hops each result took:

```go
store.AddQuad(ctx, cayley.Quad("a", "ref", "b", nil))
store.AddQuad(ctx, cayley.Quad("b", "ref", "c", nil))

// All nodes reachable from "a" through "ref" edges: b, c
p := cayley.StartPath(store, quad.String("a")).
	FollowRecursive(quad.String("ref"), -1, []string{"depth"})
```

`LabelContext` scopes the following steps to quads carrying a given label, so
the same predicate can mean different things in different subgraphs. For direct
control over iteration, call `p.BuildIterator(ctx)` and drive the scanner
yourself, resolving each result ref back to a value with `store.NameOf`.

## Development

Build and development commands are available through `make`, which wraps the
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

Useful source entry points:

- [`cayley.go`](./cayley.go): top-level Go API
- [`graph/`](./graph): quad store, writer, transaction, iterator, and backend interfaces
- [`query/`](./query): query sessions and traversal packages
- [`quad/`](./quad): quad value model and import/export formats
- [`cmd/cayley/`](./cmd/cayley): CLI and HTTP command surface
- [`examples/`](./examples): small embedded graph examples

## License

Cayley is licensed under the permissive Apache-2.0 license.
