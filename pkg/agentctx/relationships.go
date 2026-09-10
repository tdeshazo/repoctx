package agentctx

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

func graphLabel(r *ir.Repository, n int) string {
	if n < len(r.Symbols) {
		return r.String(r.Symbols[n].ID)
	}
	return r.String(r.Graph.External[n-len(r.Symbols)].Name)
}

func relationships(r *ir.Repository, b *Bundle, sources map[int]*source, limit int) ([]Relationship, int) {
	if r.Graph == nil {
		return nil, 0
	}
	selected := map[int]bool{}
	ids := map[string]bool{}
	for _, s := range b.Symbols {
		ids[s.ID] = true
	}
	for i, s := range r.Symbols {
		if ids[r.String(s.ID)] {
			selected[i] = true
		}
	}
	type arc struct {
		from, to int
		kind     ir.EdgeKind
		weight   uint32
	}
	type key struct {
		from, to int
		kind     ir.EdgeKind
	}
	seen := map[key]bool{}
	var arcs []arc
	permitted := func(n int) bool { return n >= len(r.Symbols) || sources[r.Symbols[n].File] != nil }
	add := func(from, to int, k ir.EdgeKind, w uint32) {
		a := key{from, to, k}
		if seen[a] || !permitted(from) || !permitted(to) {
			return
		}
		seen[a] = true
		arcs = append(arcs, arc{from, to, k, w})
	}
	nodes := make([]int, 0, len(selected))
	for n := range selected {
		nodes = append(nodes, n)
	}
	sort.Ints(nodes)
	// Only adjacent arcs, not a graph dump. External/unit nodes are named
	// boundaries; their neighborhoods are not traversed.
	for _, n := range nodes {
		c := r.Graph.Out
		for j := c.Offsets[n]; j < c.Offsets[n+1]; j++ {
			add(n, int(c.Targets[j]), c.Kinds[j], c.Weights[j])
		}
		c = r.Graph.In
		for j := c.Offsets[n]; j < c.Offsets[n+1]; j++ {
			add(int(c.Targets[j]), n, c.Kinds[j], c.Weights[j])
		}
	}
	sort.Slice(arcs, func(i, j int) bool {
		a, z := arcs[i], arcs[j]
		ac, zc := 0, 0
		if selected[a.from] && selected[a.to] {
			ac = 1
		}
		if selected[z.from] && selected[z.to] {
			zc = 1
		}
		if ac != zc {
			return ac > zc
		}
		af, zf := graphLabel(r, a.from), graphLabel(r, z.from)
		if af != zf {
			return af < zf
		}
		at, zt := graphLabel(r, a.to), graphLabel(r, z.to)
		if at != zt {
			return at < zt
		}
		return a.kind < z.kind
	})
	omitted := 0
	if len(arcs) > limit {
		omitted = len(arcs) - limit
		arcs = arcs[:limit]
	}
	var out []Relationship
	for _, a := range arcs {
		rel := Relationship{From: graphLabel(r, a.from), To: graphLabel(r, a.to), Kind: edgeName(a.kind), Occurrences: a.weight, Resolution: "syntactic"}
		if a.kind == ir.EdgeCalls || a.kind == ir.EdgeReferences {
			rel.Resolution = "name_heuristic"
			if a.to >= len(r.Symbols) {
				rel.Resolution = "unresolved"
			}
		}
		if a.kind == ir.EdgeDefines && a.to < len(r.Symbols) {
			s := r.Symbols[a.to]
			f := r.Files[s.File]
			rel.Sites = []Location{{File: r.String(f.Path), Span: span(f.Nodes[s.Node].Span), SHA256: f.Hash}}
		} else {
			for _, e := range r.Edges {
				if len(rel.Sites) >= 3 {
					break
				}
				if e.Kind != a.kind || sources[e.From.File] == nil {
					continue
				}
				if a.from < len(r.Symbols) && e.OwnerSymbol != a.from+1 {
					continue
				}
				if a.from >= len(r.Symbols) {
					if e.OwnerSymbol != 0 {
						continue
					}
					f := r.Files[e.From.File]
					unit := r.String(f.Unit)
					label := "unit:py:" + unit
					if f.Lang == ir.LangGo {
						dir := filepath.ToSlash(filepath.Dir(r.String(f.Path)))
						if dir != "." {
							unit = dir + "/" + unit
						}
						label = "unit:go:" + unit
					} else if f.Lang != ir.LangPython {
						label = "unit:" + r.String(f.Path)
					}
					if label != rel.From {
						continue
					}
				}
				matches := e.ToSymbol > 0 && e.ToSymbol-1 == a.to
				if e.ToSymbol == 0 && a.to >= len(r.Symbols) {
					x := r.Graph.External[a.to-len(r.Symbols)]
					label := r.String(x.Name)
					// Unresolved calls use a symbol:<name> external label,
					// while imports point at either a repository unit or an
					// external module. Keep the occurrence site attached to
					// all three boundary forms.
					switch a.kind {
					case ir.EdgeImports:
						raw := r.String(e.Text)
						matches = label == "module:"+raw
						if !matches && x.Kind == ir.GraphExternalUnit {
							matches = importUnitMatches(r, e, label, raw)
							if !matches {
								// Retain a conservative fallback for an older index whose
								// graph label cannot expose enough module information.
								matches = importArcTargets(r, e, a.to)
							}
						}
					default:
						matches = label == "symbol:"+r.String(e.Text)
					}
				}
				if !matches {
					continue
				}
				f := r.Files[e.From.File]
				rel.Sites = append(rel.Sites, Location{File: r.String(f.Path), Span: span(f.Nodes[e.From.Node].Span), SHA256: f.Hash})
			}
		}
		out = append(out, rel)
	}
	return out, omitted
}

func importUnitMatches(r *ir.Repository, e ir.Edge, label, raw string) bool {
	if e.From.File < 0 || e.From.File >= len(r.Files) {
		return false
	}
	source := r.Files[e.From.File]
	if source.Lang == ir.LangPython {
		return label == "unit:py:"+raw || strings.HasPrefix(raw, strings.TrimPrefix(label, "unit:py:")+".")
	}
	if source.Lang != ir.LangGo || !strings.HasPrefix(label, "unit:go:") {
		return false
	}
	// Current Go unit labels are unit:go:<relative-directory>/<package>.
	// The module prefix from go.mod is absent from that label, so compare the
	// relative-directory suffix of the import path. This distinguishes ordinary
	// repository imports without pretending to implement module resolution.
	key := strings.TrimPrefix(label, "unit:go:")
	dir := ""
	if slash := strings.LastIndexByte(key, '/'); slash >= 0 {
		dir = key[:slash]
	}
	if dir == "" {
		return !strings.Contains(raw, "/")
	}
	return strings.HasSuffix(raw, "/"+dir)
}

func importArcTargets(r *ir.Repository, e ir.Edge, target int) bool {
	from := -1
	if e.OwnerSymbol > 0 {
		from = e.OwnerSymbol - 1
	} else if unit := graphUnitForFile(r, e.From.File); unit >= 0 {
		from = unit
	}
	if from < 0 || from+1 >= len(r.Graph.Out.Offsets) {
		return false
	}
	for i := r.Graph.Out.Offsets[from]; i < r.Graph.Out.Offsets[from+1]; i++ {
		if r.Graph.Out.Kinds[i] == ir.EdgeImports && int(r.Graph.Out.Targets[i]) == target {
			return true
		}
	}
	return false
}

// graphUnitForFile returns the external unit node used as an occurrence's
// source when it is not owned by a symbol. Labels are derived exactly as in
// compiler.buildSymbolGraph; no additional module-resolution claim is made.
func graphUnitForFile(r *ir.Repository, file int) int {
	if file < 0 || file >= len(r.Files) {
		return -1
	}
	f := r.Files[file]
	path := filepath.ToSlash(r.String(f.Path))
	unit := r.String(f.Unit)
	label := ""
	switch f.Lang {
	case ir.LangGo:
		dir := filepath.ToSlash(filepath.Dir(path))
		if dir == "." {
			dir = ""
		}
		if dir != "" {
			unit = dir + "/" + unit
		}
		label = "unit:go:" + unit
	case ir.LangPython:
		label = "unit:py:" + unit
	default:
		label = "unit:" + path
	}
	for i, x := range r.Graph.External {
		if x.Kind == ir.GraphExternalUnit && r.String(x.Name) == label {
			return len(r.Symbols) + i
		}
	}
	return -1
}
