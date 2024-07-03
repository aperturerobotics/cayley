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

### 🗄️ Multiple Backend Stores
- **In-memory, ephemeral**
  - btree in-memory-database
- **[Key-Value Stores](./kv/kv.go)**
  - [Bolt](https://github.com/etcd-io/bbolt): Lightweight embedded K/V store
  - [Badger](https://github.com/dgraph-io/badger): Full-featured K/V store
  - [Pebble](https://github.com/cockroachdb/pebble): LevelDB/RocksDB inspired K/V store
- **[SQL Stores](./graph/sql)**
  - [CockroachDB](https://github.com/cockroachdb/cockroach)
  - [PostgreSQL](https://github.com/postgres/postgres)
  - [SQLite](https://www.sqlite.org/)
  - [MySQL](https://github.com/go-sql-driver/mysql)

### 🔍 Efficient Data Management
- Automatic indexing of quad directions (subject, predicate, object, label)
- Transactions for atomic updates

### 🔧 Powerful Query Capabilities
- Expressive query languages (Gizmo, Go API) for traversing and analyzing the graph

### 🌐 API and CLI
- RESTful API for interacting with the database
- Command-line interface for querying and managing databases

## License

Apache-2.0
