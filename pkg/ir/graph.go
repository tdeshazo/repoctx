package ir

import "sort"

// GraphExternalKind classifies non-repository-symbol graph nodes. Repository
// symbols always occupy graph node IDs [0,len(Repository.Symbols)); external
// nodes follow them in SymbolGraph.External order.
type GraphExternalKind uint8

const (
	GraphExternalUnknown GraphExternalKind = iota
	GraphExternalUnit
	GraphExternalModule
	GraphExternalSymbol
)

// GraphExternal is a compact graph node that is not backed by a repository
// Symbol. Name is a string-table reference containing a stable semantic label.
type GraphExternal struct {
	Name int               `json:"n"`
	Kind GraphExternalKind `json:"k"`
}

// CSR is a compressed-sparse-row adjacency table. Offsets has nodeCount+1
// entries. Targets, Kinds and Weights are parallel arrays. Multiple occurrence
// edges with the same (from,to,kind) are collapsed and represented by Weight.
type CSR struct {
	Offsets []uint32   `json:"o"`
	Targets []uint32   `json:"t,omitempty"`
	Kinds   []EdgeKind `json:"k,omitempty"`
	Weights []uint32   `json:"w,omitempty"`
}

// SymbolGraph is a dense graph over repository symbols plus compact external
// nodes. Out and In are both materialized so neighborhood expansion is O(degree)
// in either direction without scanning Repository.Edges.
type SymbolGraph struct {
	External []GraphExternal `json:"x,omitempty"`
	Out      CSR             `json:"o"`
	In       CSR             `json:"i"`
}

// GraphArc is a temporary/build-time graph relationship using dense node IDs.
type GraphArc struct {
	From   uint32
	To     uint32
	Kind   EdgeKind
	Weight uint32
}

// BuildCSR collapses duplicate arcs and returns deterministic forward/reverse
// CSR tables for nodeCount graph nodes.
func BuildCSR(nodeCount int, arcs []GraphArc) (CSR, CSR) {
	type key struct {
		from uint32
		to   uint32
		kind EdgeKind
	}
	weights := make(map[key]uint32, len(arcs))
	for _, a := range arcs {
		if int(a.From) >= nodeCount || int(a.To) >= nodeCount {
			continue
		}
		w := a.Weight
		if w == 0 {
			w = 1
		}
		weights[key{a.From, a.To, a.Kind}] += w
	}
	collapsed := make([]GraphArc, 0, len(weights))
	for k, w := range weights {
		collapsed = append(collapsed, GraphArc{From: k.from, To: k.to, Kind: k.kind, Weight: w})
	}
	sort.Slice(collapsed, func(i, j int) bool {
		a, b := collapsed[i], collapsed[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	out := csrFrom(nodeCount, collapsed, false)

	sort.Slice(collapsed, func(i, j int) bool {
		a, b := collapsed[i], collapsed[j]
		if a.To != b.To {
			return a.To < b.To
		}
		if a.From != b.From {
			return a.From < b.From
		}
		return a.Kind < b.Kind
	})
	in := csrFrom(nodeCount, collapsed, true)
	return out, in
}

func csrFrom(nodeCount int, arcs []GraphArc, reverse bool) CSR {
	c := CSR{Offsets: make([]uint32, nodeCount+1)}
	for _, a := range arcs {
		row := a.From
		if reverse {
			row = a.To
		}
		if int(row) < nodeCount {
			c.Offsets[row+1]++
		}
	}
	for i := 1; i < len(c.Offsets); i++ {
		c.Offsets[i] += c.Offsets[i-1]
	}
	c.Targets = make([]uint32, len(arcs))
	c.Kinds = make([]EdgeKind, len(arcs))
	c.Weights = make([]uint32, len(arcs))
	cursor := append([]uint32(nil), c.Offsets[:nodeCount]...)
	for _, a := range arcs {
		row, target := a.From, a.To
		if reverse {
			row, target = a.To, a.From
		}
		if int(row) >= nodeCount {
			continue
		}
		p := cursor[row]
		c.Targets[p], c.Kinds[p], c.Weights[p] = target, a.Kind, a.Weight
		cursor[row]++
	}
	return c
}
