package graphlog

import (
	"bytes"
	"slices"

	"github.com/aperturerobotics/cayley/graph"
	"github.com/aperturerobotics/cayley/graph/refs"
	"github.com/aperturerobotics/cayley/quad"
)

type Op interface {
	isOp()
}

var (
	_ Op = NodeUpdate{}
	_ Op = QuadUpdate{}
)

type NodeUpdate struct {
	Hash   refs.ValueHash
	Val    quad.Value
	RefInc int
}

func (NodeUpdate) isOp() {}

type QuadUpdate struct {
	Ind  int
	Quad refs.QuadHash
	Del  bool
}

func (QuadUpdate) isOp() {}

type Deltas struct {
	IncNode []NodeUpdate
	DecNode []NodeUpdate
	QuadAdd []QuadUpdate
	QuadDel []QuadUpdate
}

func InsertQuads(in []quad.Quad) *Deltas {
	hnodes := make(map[refs.ValueHash]*NodeUpdate, len(in)*2)
	quadAdd := make([]QuadUpdate, 0, len(in))
	for i, qd := range in {
		var q refs.QuadHash
		for _, dir := range quad.Directions {
			v := qd.Get(dir)
			if v == nil {
				continue
			}
			h := refs.HashOf(v)
			q.Set(dir, h)
			n := hnodes[h]
			if n == nil {
				n = &NodeUpdate{Hash: h, Val: v}
				hnodes[h] = n
			}
			n.RefInc++
		}
		quadAdd = append(quadAdd, QuadUpdate{Ind: i, Quad: q})
	}
	incNodes := make([]NodeUpdate, 0, len(hnodes))
	for _, n := range hnodes {
		incNodes = append(incNodes, *n)
	}
	slices.SortFunc(incNodes, compareNodeUpdates)
	return &Deltas{
		IncNode: incNodes,
		QuadAdd: quadAdd,
	}
}

// SplitDeltas keeps addition and removal counts separate until duplicate and
// missing quads have been resolved by the store. A node can occur in both sets.
func SplitDeltas(in []graph.Delta) *Deltas {
	added := make(map[refs.ValueHash]*NodeUpdate, len(in))
	removed := make(map[refs.ValueHash]*NodeUpdate, len(in))
	quadAdd := make([]QuadUpdate, 0, len(in))
	quadDel := make([]QuadUpdate, 0, len(in)/2)
	var nadd, ndel int
	for i, d := range in {
		dn := 0
		hnodes := added
		switch d.Action {
		case graph.Add:
			dn = +1
			nadd++
		case graph.Delete:
			hnodes = removed
			dn = -1
			ndel++
		default:
			panic("unknown action")
		}
		var q refs.QuadHash
		for _, dir := range quad.Directions {
			v := d.Quad.Get(dir)
			if v == nil {
				continue
			}
			h := refs.HashOf(v)
			q.Set(dir, h)
			n := hnodes[h]
			if n == nil {
				n = &NodeUpdate{Hash: h, Val: v}
				hnodes[h] = n
			}
			n.RefInc += dn
		}
		u := QuadUpdate{Ind: i, Quad: q, Del: d.Action == graph.Delete}
		if !u.Del {
			quadAdd = append(quadAdd, u)
		} else {
			quadDel = append(quadDel, u)
		}
	}
	incNodes := make([]NodeUpdate, 0, nadd)
	decNodes := make([]NodeUpdate, 0, ndel)
	for _, n := range added {
		incNodes = append(incNodes, *n)
	}
	for _, n := range removed {
		decNodes = append(decNodes, *n)
	}

	slices.SortFunc(incNodes, compareNodeUpdates)
	slices.SortFunc(decNodes, compareNodeUpdates)
	return &Deltas{
		IncNode: incNodes, DecNode: decNodes,
		QuadAdd: quadAdd, QuadDel: quadDel,
	}
}

func compareNodeUpdates(a, b NodeUpdate) int {
	return bytes.Compare(a.Hash[:], b.Hash[:])
}
