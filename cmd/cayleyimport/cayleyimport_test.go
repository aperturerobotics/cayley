package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/memstore"
	chttp "github.com/aperturerobotics/cayley/internal/http"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/stretchr/testify/require"
)

var expectData = []quad.Quad{
	{quad.IRI("http://example.com/alice"), quad.IRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), quad.IRI("http://xmlns.com/foaf/0.1/Person"), quad.Value(nil)},
	{quad.IRI("http://example.com/alice"), quad.IRI("http://xmlns.com/foaf/0.1/knows"), quad.IRI("http://example.com/bob"), nil},
	{quad.IRI("http://example.com/alice"), quad.IRI("http://xmlns.com/foaf/0.1/name"), quad.String("Alice"), nil},
	{quad.IRI("http://example.com/bob"), quad.IRI("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), quad.IRI("http://xmlns.com/foaf/0.1/Person"), quad.Value(nil)},
	{quad.IRI("http://example.com/bob"), quad.IRI("http://xmlns.com/foaf/0.1/knows"), quad.IRI("http://example.com/alice"), nil},
	{quad.IRI("http://example.com/bob"), quad.IRI("http://xmlns.com/foaf/0.1/name"), quad.String("Bob"), nil},
}

func allQuads(t testing.TB, qs graph.QuadStore) []quad.Quad {
	ctx := context.Background()
	it := qs.QuadsAllIterator(ctx).Iterate(ctx)
	defer it.Close()
	var out []quad.Quad
	for it.Next(ctx) {
		ref, err := it.Result(ctx)
		require.NoError(t, err)
		q, err := qs.Quad(ctx, ref)
		require.NoError(t, err)
		out = append(out, q)
	}
	require.NoError(t, it.Err())
	return out
}

func TestCayleyImport(t *testing.T) {
	qs := memstore.New()
	qw, err := graph.NewQuadWriter("single", qs, graph.Options{})
	require.NoError(t, err)
	h := &graph.Handle{QuadStore: qs, QuadWriter: qw}
	chttp.SetupRoutes(h, &chttp.Config{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		lis.Close()
	})

	srv := &http.Server{
		Addr: lis.Addr().String(),
	}
	go srv.Serve(lis)

	cmd := NewCmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	fileName := path.Join("..", "..", "data", "people.jsonld")
	cmd.SetArgs([]string{
		fileName,
		"--uri",
		fmt.Sprintf("http://%s", lis.Addr().String()),
	})
	err = cmd.Execute()
	require.NoError(t, err)
	require.Empty(t, b.String())

	allq := allQuads(t, qs)
	slices.SortFunc(allq, func(a, b quad.Quad) int {
		return strings.Compare(a.String(), b.String())
	})
	require.Equal(t, expectData, allq)
}
