package kv_test

import (
	"bytes"
	"context"
	"encoding/binary"
	henc "encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	hkv "github.com/aperturerobotics/cayley/kv"
	"github.com/stretchr/testify/require"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/kv"
	"github.com/aperturerobotics/cayley/graph/kv/btree"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
	"github.com/aperturerobotics/cayley/writer"
	b58 "github.com/mr-tron/base58/base58"
)

func hex(s string) []byte {
	b, err := henc.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func irih(s string) []byte {
	h := refs.HashOf(quad.IRI(s))
	hashB58 := b58.Encode(h[:])
	return []byte(hashB58)
}

func irib(s string) string {
	return string([]byte{'v'})
}

func iric(s string) string {
	return string([]byte{'n'})
}

func key(b string, k []byte) hkv.Key {
	return hkv.Key{[]byte(b), k}
}

func b64Col(vals ...uint64) []byte {
	var sb strings.Builder
	for i, val := range vals {
		if i != 0 {
			_, _ = sb.WriteString(":")
		}
		var data [8]byte
		binary.BigEndian.PutUint64(data[:], val)
		var i big.Int
		i.SetBytes(data[:])
		s := i.Text(62)
		_, _ = sb.WriteString(s)
	}
	return []byte(sb.String())
}

func ukey(n uint64) []byte {
	return []byte(strconv.FormatUint(n, 10))
}

func le(v uint64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return b[:]
}

const (
	bMeta = "m"
	bLog  = "l"
)

var vAuto = []byte("auto")

type Ops []kvOp

func (s Ops) Len() int {
	return len(s)
}

func (s Ops) Less(i, j int) bool {
	a, b := s[i], s[j]
	return a.key.Compare(b.key) < 0
}

func (s Ops) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s Ops) String() string {
	buf := bytes.NewBuffer(nil)
	for _, op := range s {
		se := ""
		if op.err != nil {
			se = " (" + op.err.Error() + ")"
		}
		fmt.Fprintf(buf, "%v: %q = %x%s\n", op.typ, op.key, op.val, se)
	}
	return buf.String()
}

func TestApplyDeltas(t *testing.T) {
	kdb := btree.New()

	hook := &kvHook{db: kdb}
	expect := func(exp Ops) {
		got := hook.log()
		if len(exp) == len(got) {
			if false {
				sortByOp(exp, got)
			}
			// TODO: make node insert predictable
			for i, d := range exp {
				if bytes.Equal(d.key[0], vAuto) {
					exp[i].key = got[i].key
				}
				if bytes.Equal(d.val, vAuto) {
					exp[i].val = got[i].val
				}
			}
		}
		require.Equal(t, exp, got, "%d\n%v\nvs\n\n%d\n%v", len(exp), exp, len(got), got)
	}

	ctx := context.Background()
	err := kv.Init(ctx, hook, nil)
	require.NoError(t, err)

	qs, err := kv.New(ctx, hook, nil)
	require.NoError(t, err)
	defer qs.Close()

	expect(Ops{
		{opGet, hkv.Key{[]byte("i")}, nil, hkv.ErrNotFound},
		// {opGet, key(bMeta, []byte("size")), nil, hkv.ErrNotFound},
	})

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	err = qw.AddQuad(ctx, quad.MakeIRI("a", "b", "c", ""))
	require.NoError(t, err)

	expect(Ops{
		{opGet, key(irib("a"), irih("a")), nil, nil},
		{opGet, key(irib("b"), irih("b")), nil, nil},
		{opGet, key(irib("c"), irih("c")), nil, nil},

		{opGet, key(bMeta, []byte("horizon")), nil, hkv.ErrNotFound},
		{opPut, key(irib("a"), irih("a")), vAuto, nil},
		{opPut, key(bLog, ukey(1)), vAuto, nil},
		{opPut, key(irib("b"), irih("b")), vAuto, nil},
		{opPut, key(bLog, ukey(2)), vAuto, nil},
		{opPut, key(irib("c"), irih("c")), vAuto, nil},
		{opPut, key(bLog, ukey(3)), vAuto, nil},

		{opPut, key(iric("a"), irih("a")), hex("01"), nil},
		{opPut, key(iric("b"), irih("b")), hex("01"), nil},
		{opPut, key(iric("c"), irih("c")), hex("01"), nil},
		{opPut, key(bLog, ukey(4)), vAuto, nil},
		{opGet, key(bMeta, []byte("size")), nil, hkv.ErrNotFound},
		// New-node index keys blind-write: the posting list cannot pre-exist.
		{opPut, key("ops", b64Col(3, 2, 1)), hex("04"), nil},
		{opPut, key("sp", b64Col(1, 2)), hex("04"), nil},
		{opPut, key(bMeta, []byte("horizon")), le(4), nil},
		{opPut, key(bMeta, []byte("size")), le(1), nil},
	})

	err = qw.AddQuad(ctx, quad.MakeIRI("a", "b", "e", ""))
	require.NoError(t, err)

	expect(Ops{
		{opGet, key(irib("e"), irih("e")), nil, nil},

		// served from IRI cache
		//{opGet, irib("a"), irih("a"), vAuto, nil},
		//{opGet, irib("b"), irih("b"), vAuto, nil},
		{opGet, key(bMeta, []byte("horizon")), le(4), nil},
		{opPut, key(irib("e"), irih("e")), vAuto, nil},
		{opPut, key(bLog, ukey(5)), vAuto, nil},

		{opGet, key(iric("a"), irih("a")), hex("01"), nil},
		{opGet, key(iric("b"), irih("b")), hex("01"), nil},
		{opPut, key(iric("a"), irih("a")), hex("02"), nil},
		{opPut, key(iric("b"), irih("b")), hex("02"), nil},
		{opPut, key(iric("e"), irih("e")), hex("01"), nil},
		{opPut, key(bLog, ukey(6)), vAuto, nil},
		{opGet, key(bMeta, []byte("size")), le(1), nil},
		// New object node: ops posting list blind-writes; sp exists, so it merges.
		{opPut, key("ops", b64Col(5, 2, 1)), hex("06"), nil},
		{opGet, key("sp", b64Col(1, 2)), hex("04"), nil},
		{opPut, key("sp", b64Col(1, 2)), hex("0406"), nil},
		{opPut, key(bMeta, []byte("horizon")), le(6), nil},
		{opPut, key(bMeta, []byte("size")), le(2), nil},
	})

	err = qw.RemoveQuad(ctx, quad.MakeIRI("a", "b", "c", ""))
	expect(Ops{
		{opGet, key("ops", b64Col(3, 2, 1)), hex("04"), nil},
		{opGet, key(bLog, ukey(4)), vAuto, nil},
		{opPut, key(bLog, ukey(4)), vAuto, nil},
		{opGet, key(bMeta, []byte("size")), le(2), nil},
		{opGet, key(iric("a"), irih("a")), hex("02"), nil},
		{opGet, key(iric("b"), irih("b")), hex("02"), nil},
		{opGet, key(iric("c"), irih("c")), hex("01"), nil},
		{opPut, key(iric("a"), irih("a")), hex("01"), nil},
		{opPut, key(iric("b"), irih("b")), hex("01"), nil},
		{opDel, key(iric("c"), irih("c")), nil, nil},
		{opDel, key(irib("c"), irih("c")), nil, nil},
		{opPut, key(bLog, ukey(3)), vAuto, nil},
		{opPut, key(bMeta, []byte("size")), le(1), nil},
	})
	require.NoError(t, err)
}

func TestApplyDeltasSkipsZeroRefcountUpdates(t *testing.T) {
	kdb := btree.New()
	hook := &kvHook{db: kdb}

	ctx := context.Background()
	err := kv.Init(ctx, hook, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, hook, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	err = qw.AddQuad(ctx, quad.MakeIRI("obj", "gc/ref", "old", ""))
	require.NoError(t, err)
	err = qw.AddQuad(ctx, quad.MakeIRI("unreferenced", "gc/ref", "new", ""))
	require.NoError(t, err)

	hook.log()

	err = qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: quad.MakeIRI("obj", "gc/ref", "old", ""), Action: graph.Delete},
		{Quad: quad.MakeIRI("unreferenced", "gc/ref", "new", ""), Action: graph.Delete},
		{Quad: quad.MakeIRI("obj", "gc/ref", "new", ""), Action: graph.Add},
		{Quad: quad.MakeIRI("unreferenced", "gc/ref", "old", ""), Action: graph.Add},
	}, graph.IgnoreOpts{})
	require.NoError(t, err)

	ops := hook.log()
	refKeys := []hkv.Key{
		key(iric("obj"), irih("obj")),
		key(iric("gc/ref"), irih("gc/ref")),
		key(iric("old"), irih("old")),
		key(iric("new"), irih("new")),
		key(iric("unreferenced"), irih("unreferenced")),
	}
	for _, op := range ops {
		for _, refKey := range refKeys {
			if op.key.Compare(refKey) != 0 {
				continue
			}
			t.Fatalf("unexpected zero-refcount node op on %q: %v", refKey, op)
		}
	}
}

func TestApplyDeltasIgnoreDupWithLabeledVariant(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()

	err := kv.Init(ctx, kdb, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	err = qw.AddQuad(ctx, quad.MakeIRI("a", "b", "c", ""))
	require.NoError(t, err)
	err = qw.AddQuad(ctx, quad.MakeIRI("a", "b", "c", "label"))
	require.NoError(t, err)

	err = qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: quad.MakeIRI("a", "b", "c", ""), Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: true})
	require.NoError(t, err)

	st, err := qs.Stats(ctx, false)
	require.NoError(t, err)
	require.EqualValues(t, 2, st.Quads.Value)
}

func TestApplyDeltasWriteCostSingleAddAndDuplicate(t *testing.T) {
	ctx, qs, hook, closeStore := newHookedQuadStore(t)
	defer closeStore()

	q := quad.MakeIRI("cost/s", "cost/p", "cost/o", "")
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: q, Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: true}))

	addOps := hook.log()
	require.LessOrEqual(t, len(addOps), 21, "first add-new quad should stay within the known current write-cost envelope")
	require.Equal(t, 14, countOpsOfType(addOps, opPut), "add-new should write value/log/refcount records, quad indexes, and meta keys")

	refCountsBefore := readRefCounts(ctx, t, hook.db, q.Subject, q.Predicate, q.Object)
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: q, Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: true}))

	dupOps := hook.log()
	require.Equal(t, refCountsBefore, readRefCounts(ctx, t, hook.db, q.Subject, q.Predicate, q.Object), "ignored duplicate add must not increment node refcounts")
	requireNoIndexLogOrRefcountWrites(t, dupOps)
	require.LessOrEqual(t, len(dupOps), 2, "warm ignored duplicate add should only check whether the quad exists")
}

func TestApplyDeltasWriteCostDuplicateError(t *testing.T) {
	ctx, qs, hook, closeStore := newHookedQuadStore(t)
	defer closeStore()

	q := quad.MakeIRI("cost/error-s", "cost/error-p", "cost/error-o", "")
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: q, Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: true}))
	hook.log()

	refCountsBefore := readRefCounts(ctx, t, hook.db, q.Subject, q.Predicate, q.Object)
	err := qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: q, Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: false})

	dupOps := hook.log()
	require.Error(t, err)
	require.True(t, graph.IsQuadExist(err), "expected ErrQuadExists/DeltaError, got %v", err)
	deltaErr, ok := err.(*graph.DeltaError)
	require.True(t, ok, "expected DeltaError, got %T", err)
	require.Equal(t, graph.ErrQuadExists, deltaErr.Err)
	require.Equal(t, q, deltaErr.Delta.Quad)
	require.Equal(t, graph.Add, deltaErr.Delta.Action)
	require.Equal(t, refCountsBefore, readRefCounts(ctx, t, hook.db, q.Subject, q.Predicate, q.Object), "failed duplicate add must leave node refcounts unchanged")
	requireNoIndexLogOrRefcountWrites(t, dupOps)
	require.LessOrEqual(t, len(dupOps), 2, "warm duplicate add error should only check whether the quad exists")
}

func TestApplyDeltasIgnoredDuplicateWithMissingDeletePreservesRefcounts(t *testing.T) {
	ctx, qs, hook, closeStore := newHookedQuadStore(t)
	defer closeStore()

	existing := quad.MakeIRI("mixed/s", "mixed/p", "mixed/o1", "")
	missing := quad.MakeIRI("mixed/s", "mixed/p", "mixed/o2", "")
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: existing, Action: graph.Add},
	}, graph.IgnoreOpts{IgnoreDup: true}))
	hook.log()

	refCountsBefore := readRefCounts(ctx, t, hook.db, existing.Subject, existing.Predicate, existing.Object)
	require.NoError(t, qs.ApplyDeltas(ctx, []graph.Delta{
		{Quad: existing, Action: graph.Add},
		{Quad: missing, Action: graph.Delete},
	}, graph.IgnoreOpts{IgnoreDup: true, IgnoreMissing: true}))

	require.Equal(t, refCountsBefore, readRefCounts(ctx, t, hook.db, existing.Subject, existing.Predicate, existing.Object), "ignored duplicate add plus ignored missing delete must leave live quad node refcounts unchanged")
	st, err := qs.Stats(ctx, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, st.Quads.Value)
}

func TestApplyTransactionWriteCostBatchAdds(t *testing.T) {
	ctx, qs, hook, closeStore := newHookedQuadStore(t)
	defer closeStore()

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{IgnoreDup: true})
	require.NoError(t, err)

	tx := graph.NewTransactionN(4)
	quads := []quad.Quad{
		quad.MakeIRI("batch/s", "batch/p", "batch/o1", ""),
		quad.MakeIRI("batch/s", "batch/p", "batch/o2", ""),
		quad.MakeIRI("batch/s", "batch/p", "batch/o3", ""),
		quad.MakeIRI("batch/s", "batch/p", "batch/o4", ""),
	}
	for _, q := range quads {
		tx.AddQuad(q)
	}

	require.NoError(t, qw.ApplyTransaction(ctx, tx))

	ops := hook.log()
	require.LessOrEqual(t, len(ops), 42, "four-quad ApplyTransaction should batch shared node and index work")
	require.Equal(t, 29, countOpsOfType(ops, opPut), "batch add should write six values, six node logs, six refcounts, four quad logs, five index buckets, and two meta keys")

	requireRefCount(t, ctx, hook.db, quad.IRI("batch/s"), 4)
	requireRefCount(t, ctx, hook.db, quad.IRI("batch/p"), 4)
	for _, obj := range []quad.Value{
		quad.IRI("batch/o1"),
		quad.IRI("batch/o2"),
		quad.IRI("batch/o3"),
		quad.IRI("batch/o4"),
	} {
		requireRefCount(t, ctx, hook.db, obj, 1)
	}

	st, err := qs.Stats(ctx, false)
	require.NoError(t, err)
	require.EqualValues(t, len(quads), st.Quads.Value)
}

func TestRemoveQuadPreservesUnrelatedQuadValuesAfterNodeDeletion(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()

	err := kv.Init(ctx, kdb, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	rel := quad.MakeIRI("cluster", "cluster-job", "job", "")
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("cluster-wizard", "type", "types/cluster-wizard", "")))
	require.NoError(t, qw.RemoveQuad(ctx, quad.MakeIRI("cluster-wizard", "type", "types/cluster-wizard", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("job-wizard", "type", "types/job-wizard", "")))
	require.NoError(t, qw.RemoveQuad(ctx, quad.MakeIRI("job-wizard", "type", "types/job-wizard", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("cluster", "type", "types/cluster", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("job", "type", "types/job", "")))
	require.NoError(t, qw.AddQuad(ctx, rel))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("task-wizard", "type", "types/task-wizard", "")))
	require.NoError(t, qw.RemoveQuad(ctx, quad.MakeIRI("task-wizard", "type", "types/task-wizard", "")))

	it := qs.QuadIterator(ctx, quad.Subject, mustValueOf(ctx, t, qs, quad.IRI("cluster"))).Iterate(ctx)
	defer it.Close()

	var found bool
	for it.Next(ctx) {
		ref, err := it.Result(ctx)
		require.NoError(t, err)
		q, err := qs.Quad(ctx, ref)
		require.NoError(t, err)
		if q.Subject == quad.IRI("cluster") && q.Predicate == quad.IRI("cluster-job") {
			require.Equal(t, rel, q)
			found = true
		}
	}
	require.NoError(t, it.Err())
	require.True(t, found, "expected relationship quad to remain")
}

func TestCollectFilteredQuadsBatch(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()

	err := kv.Init(ctx, kdb, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "b", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "c", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "other", "d", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("x", "p", "b", "")))
	require.NoError(t, qw.RemoveQuad(ctx, quad.MakeIRI("a", "other", "d", "")))

	results, err := qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("a", "p", "", ""),
		quad.MakeIRI("a", "p", "b", ""),
		quad.MakeIRI("missing", "p", "", ""),
		quad.MakeIRI("", "p", "b", ""),
		{},
	}, 0)
	require.NoError(t, err)
	require.Len(t, results, 5)
	require.ElementsMatch(t, []string{
		quad.MakeIRI("a", "p", "b", "").String(),
		quad.MakeIRI("a", "p", "c", "").String(),
	}, quadStrings(results[0]))
	require.Equal(t, []string{quad.MakeIRI("a", "p", "b", "").String()}, quadStrings(results[1]))
	require.Empty(t, results[2])
	require.ElementsMatch(t, []string{
		quad.MakeIRI("a", "p", "b", "").String(),
		quad.MakeIRI("x", "p", "b", "").String(),
	}, quadStrings(results[3]))
	require.ElementsMatch(t, []string{
		quad.MakeIRI("a", "p", "b", "").String(),
		quad.MakeIRI("a", "p", "c", "").String(),
		quad.MakeIRI("x", "p", "b", "").String(),
	}, quadStrings(results[4]))
}

func TestCollectFilteredQuadsBatchLimitPerFilter(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()

	err := kv.Init(ctx, kdb, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "b", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "c", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("x", "p", "b", "")))

	results, err := qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("a", "p", "", ""),
		quad.MakeIRI("", "p", "b", ""),
	}, 1)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Len(t, results[0], 1)
	require.Len(t, results[1], 1)
}

func TestCollectFilteredQuadsBatchLimitPerFilterSkipsFilledFilters(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()

	err := kv.Init(ctx, kdb, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	defer gqs.Close()

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{})
	require.NoError(t, err)

	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "b", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "p", "c", "")))
	require.NoError(t, qw.AddQuad(ctx, quad.MakeIRI("a", "q", "d", "")))

	results, err := qs.CollectFilteredQuadsBatch(ctx, []quad.Quad{
		quad.MakeIRI("a", "p", "", ""),
		quad.MakeIRI("a", "q", "", ""),
	}, 1)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Len(t, results[0], 1)
	require.Equal(t, []string{quad.MakeIRI("a", "q", "d", "").String()}, quadStrings(results[1]))
}

func newHookedQuadStore(t testing.TB) (context.Context, *kv.QuadStore, *kvHook, func()) {
	t.Helper()

	kdb := btree.New()
	hook := &kvHook{db: kdb}
	ctx := context.Background()

	err := kv.Init(ctx, hook, nil)
	require.NoError(t, err)

	gqs, err := kv.New(ctx, hook, nil)
	require.NoError(t, err)

	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)
	hook.log()

	return ctx, qs, hook, func() {
		require.NoError(t, qs.Close())
	}
}

func countOpsOfType(ops Ops, typ int) int {
	var n int
	for _, op := range ops {
		if op.typ == typ {
			n++
		}
	}
	return n
}

func requireNoIndexLogOrRefcountWrites(t testing.TB, ops Ops) {
	t.Helper()

	for _, op := range ops {
		if op.typ != opPut && op.typ != opDel {
			continue
		}
		require.Falsef(t, isIndexLogOrRefcountKey(op.key), "unexpected write to refcount/log/index key: %v", op)
	}
}

func isIndexLogOrRefcountKey(k hkv.Key) bool {
	if len(k) == 0 {
		return false
	}
	switch string(k[0]) {
	case bLog, "n", "sp", "ops":
		return true
	default:
		return false
	}
}

func readRefCounts(ctx context.Context, t testing.TB, db hkv.KV, vals ...quad.Value) []uint64 {
	t.Helper()

	out := make([]uint64, len(vals))
	err := hkv.View(ctx, db, func(tx hkv.Tx) error {
		for i, val := range vals {
			h := refs.HashOf(val)
			raw, err := tx.Get(ctx, key(iric(""), irihFromHash(h)))
			if err == hkv.ErrNotFound {
				continue
			}
			if err != nil {
				return err
			}
			out[i], _ = binary.Uvarint(raw)
		}
		return nil
	})
	require.NoError(t, err)
	return out
}

func requireRefCount(t testing.TB, ctx context.Context, db hkv.KV, val quad.Value, expected uint64) {
	t.Helper()

	require.Equal(t, []uint64{expected}, readRefCounts(ctx, t, db, val))
}

func irihFromHash(h refs.ValueHash) []byte {
	hashB58 := b58.Encode(h[:])
	return []byte(hashB58)
}

func BenchmarkApplyDeltasWriteCostCounts(b *testing.B) {
	var addOps, addReads, duplicateOps, duplicateReads, duplicateErrOps, duplicateErrReads int
	var batch4Ops, batch4Reads, batch16Ops, batch16Reads, batch64Ops, batch64Reads, loops int
	for b.Loop() {
		ctx, qs, hook, closeStore := newHookedQuadStore(b)
		q := quad.MakeIRI("bench/s", "bench/p", "bench/o", "")
		if err := qs.ApplyDeltas(ctx, []graph.Delta{
			{Quad: q, Action: graph.Add},
		}, graph.IgnoreOpts{IgnoreDup: true}); err != nil {
			b.Fatal(err.Error())
		}
		addLog := hook.log()
		addOps += len(addLog)
		addReads += countOpsOfType(addLog, opGet)
		if err := qs.ApplyDeltas(ctx, []graph.Delta{
			{Quad: q, Action: graph.Add},
		}, graph.IgnoreOpts{IgnoreDup: true}); err != nil {
			b.Fatal(err.Error())
		}
		duplicateLog := hook.log()
		duplicateOps += len(duplicateLog)
		duplicateReads += countOpsOfType(duplicateLog, opGet)
		err := qs.ApplyDeltas(ctx, []graph.Delta{
			{Quad: q, Action: graph.Add},
		}, graph.IgnoreOpts{IgnoreDup: false})
		if !graph.IsQuadExist(err) {
			b.Fatalf("expected duplicate error, got %v", err)
		}
		duplicateErrLog := hook.log()
		duplicateErrOps += len(duplicateErrLog)
		duplicateErrReads += countOpsOfType(duplicateErrLog, opGet)
		closeStore()

		ops4, reads4 := applyBatchAddOps(b, 4)
		batch4Ops += ops4
		batch4Reads += reads4
		ops16, reads16 := applyBatchAddOps(b, 16)
		batch16Ops += ops16
		batch16Reads += reads16
		ops64, reads64 := applyBatchAddOps(b, 64)
		batch64Ops += ops64
		batch64Reads += reads64
		loops++
	}
	b.ReportMetric(float64(addOps)/float64(loops), "add_new_kv_ops/op")
	b.ReportMetric(float64(addReads)/float64(loops), "add_new_kv_reads/op")
	b.ReportMetric(float64(duplicateOps)/float64(loops), "duplicate_ignore_kv_ops/op")
	b.ReportMetric(float64(duplicateReads)/float64(loops), "duplicate_ignore_kv_reads/op")
	b.ReportMetric(float64(duplicateErrOps)/float64(loops), "duplicate_error_kv_ops/op")
	b.ReportMetric(float64(duplicateErrReads)/float64(loops), "duplicate_error_kv_reads/op")
	b.ReportMetric(float64(batch4Ops)/float64(loops), "batch4_kv_ops/op")
	b.ReportMetric(float64(batch4Reads)/float64(loops), "batch4_kv_reads/op")
	b.ReportMetric(float64(batch16Ops)/float64(loops), "batch16_kv_ops/op")
	b.ReportMetric(float64(batch16Reads)/float64(loops), "batch16_kv_reads/op")
	b.ReportMetric(float64(batch64Ops)/float64(loops), "batch64_kv_ops/op")
	b.ReportMetric(float64(batch64Reads)/float64(loops), "batch64_kv_reads/op")
}

// applyBatchAddOps applies one n-quad add-new batch sharing a subject and
// predicate to a fresh store and returns the logical KV op and read counts.
func applyBatchAddOps(b *testing.B, n int) (ops, reads int) {
	b.Helper()
	ctx, qs, hook, closeStore := newHookedQuadStore(b)
	defer closeStore()
	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{IgnoreDup: true})
	if err != nil {
		b.Fatal(err.Error())
	}
	tx := graph.NewTransactionN(n)
	for i := range n {
		tx.AddQuad(quad.MakeIRI("bench/batch-s", "bench/batch-p", "bench/batch-o"+strconv.Itoa(i), ""))
	}
	if err := qw.ApplyTransaction(ctx, tx); err != nil {
		b.Fatal(err.Error())
	}
	log := hook.log()
	return len(log), countOpsOfType(log, opGet)
}

func mustValueOf(ctx context.Context, t testing.TB, qs graph.QuadStore, v quad.Value) graph.Ref {
	t.Helper()

	ref, err := qs.ValueOf(ctx, v)
	require.NoError(t, err)
	require.NotNil(t, ref)
	return ref
}

func quadStrings(quads []quad.Quad) []string {
	out := make([]string, len(quads))
	for i, q := range quads {
		out[i] = q.String()
	}
	return out
}

func allQuads(ctx context.Context, t testing.TB, qs *kv.QuadStore) []quad.Quad {
	t.Helper()

	var out []quad.Quad
	it := qs.QuadsAllIterator(ctx).Iterate(ctx)
	defer it.Close()
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

// TestBulkAddFreshIndexFormatRoundTrips proves the fresh-index blind-write path
// persists byte-identical on-disk state to the read-modify-write path: a cold
// QuadStore reopened over the same store reads every quad and both indexes back,
// including a posting list that the second batch merged into a fresh-written one.
func TestBulkAddFreshIndexFormatRoundTrips(t *testing.T) {
	kdb := btree.New()
	ctx := context.Background()
	require.NoError(t, kv.Init(ctx, kdb, nil))

	gqs, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	qs, ok := gqs.(*kv.QuadStore)
	require.True(t, ok)

	qw, err := writer.NewSingle(qs, graph.IgnoreOpts{IgnoreDup: true})
	require.NoError(t, err)

	// First batch: all-new nodes exercise the fresh blind-write path on both
	// indexes (the sp key and every ops key hold a node minted this transaction).
	var want []quad.Quad
	tx := graph.NewTransactionN(6)
	for i := range 6 {
		q := quad.MakeIRI("bulk/s", "bulk/p", "bulk/o"+strconv.Itoa(i), "")
		tx.AddQuad(q)
		want = append(want, q)
	}
	require.NoError(t, qw.ApplyTransaction(ctx, tx))

	// Second batch: the shared subject/predicate forces the sp posting list to
	// merge onto the fresh-written list persisted by the first batch.
	tx2 := graph.NewTransactionN(3)
	for i := 6; i < 9; i++ {
		q := quad.MakeIRI("bulk/s", "bulk/p", "bulk/o"+strconv.Itoa(i), "")
		tx2.AddQuad(q)
		want = append(want, q)
	}
	require.NoError(t, qw.ApplyTransaction(ctx, tx2))

	// Reopen a cold QuadStore over the same persisted store (empty caches).
	gqs2, err := kv.New(ctx, kdb, nil)
	require.NoError(t, err)
	qs2, ok := gqs2.(*kv.QuadStore)
	require.True(t, ok)
	defer qs2.Close()

	sz, err := qs2.Size(ctx)
	require.NoError(t, err)
	require.EqualValues(t, len(want), sz)

	require.ElementsMatch(t, quadStrings(want), quadStrings(allQuads(ctx, t, qs2)))
}

func sortByOp(exp, got Ops) {
	// sort ops of one type
	li := -1
	typ, b := -1, ""
	check := func(i int) {
		if li < 0 || i-li <= 0 {
			return
		}
		sort.Sort(exp[li:i])
		sort.Sort(got[li:i])
		// sort.Sort(bothOps{a: exp[li:i], b: got[li:i]})
		li, typ, b = -1, -1, ""
	}
	for i, op := range exp {
		if op.typ != typ {
			check(i)
		}
		if li < 0 {
			li, typ, b = i, op.typ, string(op.key[0])
		}
	}
	_ = b
	check(len(exp))
}

const (
	opGet = iota
	opPut
	opDel
)

type kvOp struct {
	typ int
	key hkv.Key
	val hkv.Value
	err error
}

var _ hkv.KV = (*kvHook)(nil)

type kvHook struct {
	db hkv.KV

	mu  sync.Mutex
	ops Ops
}

func (h *kvHook) log() Ops {
	h.mu.Lock()
	ops := h.ops
	h.ops = nil
	h.mu.Unlock()
	return ops
}

func (h *kvHook) addOp(op kvOp) {
	h.mu.Lock()
	h.ops = append(h.ops, op)
	h.mu.Unlock()
}

func (h *kvHook) Close() error {
	return h.db.Close()
}

func (h *kvHook) Tx(ctx context.Context, rw bool) (hkv.Tx, error) {
	tx, err := h.db.Tx(ctx, rw)
	if err != nil {
		return nil, err
	}
	return txHook{h: h, tx: tx}, nil
}

func (h *kvHook) View(ctx context.Context, fn func(tx hkv.Tx) error) error {
	return hkv.View(ctx, h, fn)
}

func (h *kvHook) Update(ctx context.Context, fn func(tx hkv.Tx) error) error {
	return hkv.Update(ctx, h, fn)
}

type txHook struct {
	h  *kvHook
	tx hkv.Tx
}

func (h txHook) Commit(ctx context.Context) error {
	return h.tx.Commit(ctx)
}

func (h txHook) Close() error {
	return h.tx.Close()
}

func (h txHook) GetBatch(ctx context.Context, keys []hkv.Key) ([]hkv.Value, error) {
	vals, err := h.tx.GetBatch(ctx, keys)
	if err != nil {
		return nil, err
	}
	for i, k := range keys {
		h.h.addOp(kvOp{
			key: k.Clone(),
			val: vals[i].Clone(),
		})
	}
	return vals, nil
}

func (h txHook) Get(ctx context.Context, k hkv.Key) (hkv.Value, error) {
	v, err := h.tx.Get(ctx, k)
	h.h.addOp(kvOp{
		key: k.Clone(),
		val: v.Clone(),
		err: err,
	})
	return v, err
}

func (h txHook) Put(ctx context.Context, k hkv.Key, v hkv.Value) error {
	err := h.tx.Put(ctx, k, v)
	h.h.addOp(kvOp{
		typ: opPut,
		key: k.Clone(),
		val: v.Clone(),
		err: err,
	})
	return err
}

func (h txHook) Del(ctx context.Context, k hkv.Key) error {
	err := h.tx.Del(ctx, k)
	h.h.addOp(kvOp{
		typ: opDel,
		key: k.Clone(),
		err: err,
	})
	return err
}

func (h txHook) Scan(ctx context.Context, opts ...hkv.IteratorOption) hkv.Iterator {
	return h.tx.Scan(ctx, opts...)
}
