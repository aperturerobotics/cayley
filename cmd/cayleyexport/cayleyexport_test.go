package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/memstore"
	chttp "github.com/aperturerobotics/cayley/internal/http"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/jsonld"
	"github.com/stretchr/testify/require"
)

var testData = []quad.Quad{
	{
		Subject:   quad.IRI("http://example.com/alice"),
		Predicate: quad.IRI("http://example.com/likes"),
		Object:    quad.IRI("http://example.com/bob"),
		Label:     nil,
	},
}

func serializeTestData() string {
	buf := bytes.NewBuffer(nil)
	w := jsonld.NewWriter(buf)
	_, _ = w.WriteQuads(context.Background(), testData)
	_ = w.Close()
	return buf.String()
}

func TestCayleyExport(t *testing.T) {
	qs := memstore.New(testData...)
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
	cmd.SetArgs([]string{
		"--uri",
		fmt.Sprintf("http://%s", lis.Addr().String()),
	})
	err = cmd.Execute()
	require.NoError(t, err)
	data := serializeTestData()
	require.NotEmpty(t, data)
	require.Equal(t, data, b.String())
}
