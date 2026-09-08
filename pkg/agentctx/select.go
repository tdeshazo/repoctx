package agentctx

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

type candidate struct {
	node   int
	reason Reason
	seed   bool
}

func terms(s string) []string {
	var out []string
	var word []rune
	var prev rune
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if len(word) > 0 {
				out = append(out, strings.ToLower(string(word)))
				word = nil
			}
		} else {
			if unicode.IsUpper(r) && unicode.IsLower(prev) && len(word) > 0 {
				out = append(out, strings.ToLower(string(word)))
				word = nil
			}
			word = append(word, r)
		}
		prev = r
	}
	if len(word) > 0 {
		out = append(out, strings.ToLower(string(word)))
	}
	stop := map[string]bool{"a": true, "an": true, "the": true, "to": true, "in": true, "of": true, "for": true, "and": true, "fix": true, "change": true, "how": true, "does": true, "is": true}
	seen := map[string]bool{}
	var clean []string
	for _, x := range out {
		if !stop[x] && !seen[x] {
			seen[x] = true
			clean = append(clean, x)
		}
	}
	return clean
}
func choose(r *ir.Repository, o Options, sources map[int]*source) ([]candidate, []string, bool, error) {
	var seeds []candidate
	byID := map[string]int{}
	for i, s := range r.Symbols {
		if sources[s.File] != nil {
			byID[r.String(s.ID)] = i
		}
	}
	if len(o.Symbols) > 0 {
		seen := map[int]bool{}
		for _, id := range o.Symbols {
			i, ok := byID[id]
			if !ok {
				return nil, nil, false, fmt.Errorf("requested symbol not found in permitted index: %q", id)
			}
			if !seen[i] {
				seen[i] = true
				seeds = append(seeds, candidate{i, Reason{Strategy: "explicit_symbol", Score: 100000}, true})
			}
		}
	} else {
		words := terms(o.Query)
		var ranked []candidate
		for i, s := range r.Symbols {
			if sources[s.File] == nil {
				continue
			}
			id := r.String(s.ID)
			name := strings.ToLower(r.String(s.Name))
			qualified := strings.ToLower(id)
			p := strings.ToLower(r.String(r.Files[s.File].Path))
			score := 0
			q := strings.ToLower(strings.TrimSpace(o.Query))
			if q == name || q == qualified {
				score += 10000
			}
			for _, t := range words {
				if t == name {
					score += 100
				} else if strings.Contains(name, t) {
					score += 50
				}
				if strings.Contains(qualified, t) {
					score += 20
				}
				if strings.Contains(p, t) {
					score += 5
				}
			}
			if score > 0 {
				ranked = append(ranked, candidate{i, Reason{Strategy: "lexical_seed", Score: score}, true})
			}
		}
		sort.Slice(ranked, func(i, j int) bool {
			if ranked[i].reason.Score != ranked[j].reason.Score {
				return ranked[i].reason.Score > ranked[j].reason.Score
			}
			return r.String(r.Symbols[ranked[i].node].ID) < r.String(r.Symbols[ranked[j].node].ID)
		})
		if len(ranked) > 0 {
			threshold := ranked[0].reason.Score / 2
			for _, c := range ranked {
				if len(seeds) >= 3 || len(seeds) >= o.MaxSymbols || len(seeds) >= o.MaxCandidates || c.reason.Score < threshold {
					break
				}
				seeds = append(seeds, c)
			}
		}
	}
	if len(seeds) == 0 {
		return nil, nil, false, fmt.Errorf("no matching symbols; use a name, path, or exact -symbol ID")
	}
	if len(seeds) > o.MaxSymbols || len(seeds) > o.MaxCandidates {
		return nil, nil, false, fmt.Errorf("seed count exceeds symbol/candidate limit")
	}
	wanted := map[ir.EdgeKind]bool{}
	for _, k := range o.Relations {
		wanted[k] = true
	}
	selected := map[int]candidate{}
	var queue []int
	var ids []string
	for _, c := range seeds {
		selected[c.node] = c
		queue = append(queue, c.node)
		ids = append(ids, r.String(r.Symbols[c.node].ID))
	}
	limited := false
	scanned := 0
	if r.Graph != nil {
		for q := 0; q < len(queue); q++ {
			cur := selected[queue[q]]
			if cur.reason.Depth >= o.Depth {
				continue
			}
			type direction struct {
				name string
				csr  ir.CSR
			}
			dirs := []direction{}
			if o.Direction != "in" {
				dirs = append(dirs, direction{"out", r.Graph.Out})
			}
			if o.Direction != "out" {
				dirs = append(dirs, direction{"in", r.Graph.In})
			}
			for _, d := range dirs {
				for j := d.csr.Offsets[cur.node]; j < d.csr.Offsets[cur.node+1]; j++ {
					scanned++
					if scanned > 100000 {
						limited = true
						break
					}
					n := int(d.csr.Targets[j])
					k := d.csr.Kinds[j]
					// Do not traverse unit hubs or globally merged unresolved names. Such
					// expansion creates spurious relationships between unrelated packages.
					if n >= len(r.Symbols) || !wanted[k] || sources[r.Symbols[n].File] == nil {
						continue
					}
					if _, ok := selected[n]; ok {
						continue
					}
					if len(selected) >= o.MaxCandidates {
						limited = true
						continue
					}
					next := candidate{node: n, reason: Reason{Strategy: "graph_neighbor", Depth: cur.reason.Depth + 1, Score: cur.reason.Score / 2, Via: r.String(r.Symbols[cur.node].ID), Relation: edgeName(k), Direction: d.name}}
					selected[n] = next
					queue = append(queue, n)
				}
				if scanned > 100000 {
					break
				}
			}
			if scanned > 100000 {
				break
			}
		}
	}
	var rest []candidate
	for _, c := range selected {
		if !c.seed {
			rest = append(rest, c)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		a, b := rest[i], rest[j]
		if a.reason.Depth != b.reason.Depth {
			return a.reason.Depth < b.reason.Depth
		}
		if a.reason.Score != b.reason.Score {
			return a.reason.Score > b.reason.Score
		}
		return r.String(r.Symbols[a.node].ID) < r.String(r.Symbols[b.node].ID)
	})
	return append(seeds, rest...), ids, limited, nil
}
func edgeName(k ir.EdgeKind) string {
	return map[ir.EdgeKind]string{ir.EdgeDefines: "defines", ir.EdgeCalls: "calls", ir.EdgeImports: "imports", ir.EdgeReferences: "references"}[k]
}
func kindName(k ir.SymbolKind) string {
	return map[ir.SymbolKind]string{ir.SymModule: "module", ir.SymType: "type", ir.SymFunction: "function", ir.SymMethod: "method", ir.SymVariable: "variable", ir.SymConstant: "constant"}[k]
}
func languageName(k ir.Language) string {
	switch k {
	case ir.LangGo:
		return "go"
	case ir.LangPython:
		return "python"
	case ir.LangHTML:
		return "html"
	case ir.LangCSS:
		return "css"
	case ir.LangJavaScript:
		return "javascript"
	case ir.LangTypeScript:
		return "typescript"
	case ir.LangTSX:
		return "tsx"
	case ir.LangMarkdown:
		return "markdown"
	default:
		return "unknown"
	}
}
