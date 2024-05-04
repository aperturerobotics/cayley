# Quad formats for Go

This library provides encoding and decoding support for NQuad/NTriple-compatible formats.

Forked from the [upstream project].

[upstream project]: https://github.com/cayleygraph/quad

## Supported formats

ID  | Name | Read | Write | Ext
--- | ---- | ---- | ----- | ---
`nquads` | NQuads | + | + | `.nq`, `.nt`
`jsonld` | JSON-LD | + | + | `.jsonld`
`graphviz` | DOT/Graphviz | - | + | `.gv`, `.dot`
`gml` | GML | - | + | `.gml`
`graphml` | GraphML | - | + | `.graphml`
`pquads` | ProtoQuads | + | + | `.pq`
`json` | JSON | + | + | `.json`
`json-stream` | JSON Stream | + | + | -
