package cayleyhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/memstore"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/quad/jsonld"
	"github.com/aperturerobotics/cayley/writer"
	"github.com/stretchr/testify/require"
)

func makeHandle(t testing.TB, quads ...quad.Quad) *graph.Handle {
	qs := memstore.New(quads...)
	wr, err := writer.NewSingleReplication(qs, nil)
	require.NoError(t, err)
	return &graph.Handle{
		QuadStore:  qs,
		QuadWriter: wr,
	}
}

func makeServerV2(t testing.TB, quads ...quad.Quad) *APIv2 {
	h := makeHandle(t, quads...)
	return NewAPIv2(h)
}

func writeQuads(ctx context.Context, q []quad.Quad, w io.Writer) error {
	writer := jsonld.NewWriter(w)
	reader := quad.NewReader(quads)
	_, err := quad.Copy(ctx, writer, reader)
	writer.Close()
	return err
}

func newQuadsBuffer(ctx context.Context, quads []quad.Quad) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	err := writeQuads(ctx, quads, buf)
	return buf, err
}

var mime = quad.FormatByName("jsonld").Mime[0]

var quads = []quad.Quad{
	quad.MakeIRI("http://example.com/bob", "http://example.com/likes", "http://example.com/alice", ""),
	quad.MakeIRI("http://example.com/alice", "http://example.com/likes", "http://example.com/bob", ""),
}

func TestV2Write(t *testing.T) {
	api := makeServerV2(t)
	ctx := context.Background()
	buf, err := newQuadsBuffer(ctx, quads)

	req, err := http.NewRequest(http.MethodGet, prefix+"/write", buf)
	require.NoError(t, err)
	req.Header.Set(hdrContentType, mime)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.ServeWrite)
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	expectedResponse := newWriteResponse(len(quads))

	var response writeResponse
	json.Unmarshal(rr.Body.Bytes(), &response)

	require.Equal(t, expectedResponse, response)
}

func TestV2Read(t *testing.T) {
	api := makeServerV2(t, quads...)
	buf := bytes.NewBuffer(nil)

	req, err := http.NewRequest(http.MethodGet, prefix+"/read", buf)
	require.NoError(t, err)
	req.Header.Set(hdrAccept, mime)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.ServeRead)
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	reader := jsonld.NewReader(rr.Body)
	ctx := context.Background()
	receivedQuads, err := quad.ReadAll(ctx, reader)
	require.NoError(t, err)
	sort.Sort(quad.ByQuadString(receivedQuads))
	sort.Sort(quad.ByQuadString(quads))
	require.Equal(t, quads, receivedQuads)
}

func TestV2Delete(t *testing.T) {
	api := makeServerV2(t, quads...)
	ctx := context.Background()
	buf, err := newQuadsBuffer(ctx, quads)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, prefix+"/delete", buf)
	require.NoError(t, err)
	req.Header.Set(hdrContentType, mime)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.ServeDelete)
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestV2GetNamespaceRules(t *testing.T) {
	api := makeServerV2(t)
	buf := bytes.NewBuffer(nil)
	req, err := http.NewRequest(http.MethodGet, prefix+"/namespace-rules", buf)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.ServeNamespaceRules)
	handler.ServeHTTP(rr, req)
	var rules []NamespaceRule
	err = json.Unmarshal(rr.Body.Bytes(), &rules)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, contentTypeJSON, rr.Header().Get(hdrContentType))
	require.NotEmpty(t, rules)
}

func TestV2PostNamespaceRules(t *testing.T) {
	api := makeServerV2(t)
	rule := NamespaceRule{
		Prefix:    "foaf",
		Namespace: "http://xmlns.com/foaf/0.1/",
	}
	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	encoder.Encode(rule)
	req, err := http.NewRequest(http.MethodPost, prefix+"/namespace-rules", buf)
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.ServeNamespaceRules)
	handler.ServeHTTP(rr, req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, rr.Code)
	require.Empty(t, rr.Body.Bytes())

	// Check effect
	req, err = http.NewRequest(http.MethodGet, prefix+"/namespace-rules", buf)
	require.NoError(t, err)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(api.ServeNamespaceRules)
	handler.ServeHTTP(rr, req)
	var rules []NamespaceRule
	err = json.Unmarshal(rr.Body.Bytes(), &rules)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, contentTypeJSON, rr.Header().Get(hdrContentType))
	require.Contains(t, rules, rule)
}
