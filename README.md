# Cayley

[![GoDoc Widget]][GoDoc] [![Go Report Card Widget]][Go Report Card]

[GoDoc]: https://godoc.org/github.com/aperturerobotics/cayley
[GoDoc Widget]: https://godoc.org/github.com/aperturerobotics/cayley?status.svg
[Go Report Card Widget]: https://goreportcard.com/badge/github.com/aperturerobotics/cayley
[Go Report Card]: https://goreportcard.com/report/github.com/aperturerobotics/cayley

Cayley is an open-source database for [Linked Data](https://www.w3.org/standards/semanticweb/data). It is inspired by the graph database behind Google's [Knowledge Graph](https://en.wikipedia.org/wiki/Knowledge_Graph) (formerly [Freebase](https://en.wikipedia.org/wiki/Freebase_(database))).

**This is a fork of the [upstream project].**

[upstream project]: https://github.com/cayleygraph/cayley

## Features

- Built-in query editor, visualizer and REPL
- Multiple query languages:
  - [Gizmo](./docs/gizmoapi.md): query language inspired by [Gremlin](https://tinkerpop.apache.org/gremlin.html)
  - [GraphQL](./docs/graphql.md)-inspired query language
  - [MQL](./docs/mql.md): simplified version for [Freebase](https://en.wikipedia.org/wiki/Freebase_(database)) fans
- Modular: easy to connect to your favorite programming languages and back-end stores
- Production ready: well tested and used by various companies for their production workloads
- Fast: optimized specifically for usage in applications

### Performance

Rough performance testing shows that, on 2014 consumer hardware and an average disk, 134m quads in LevelDB is no problem and a multi-hop intersection query -- films starring X and Y -- takes ~150ms.

## License

Apache-2.0
