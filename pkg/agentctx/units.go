package agentctx

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

type retrievalUnit struct {
	Unit
	file       int
	start, end int
}

// Units are derived only after permitted indexed source has been hash-verified.
// No additional source text or index is persisted. IDs bind path, hash, kind,
// and physical offsets, so repeated headings cannot collide.
func retrievalUnits(r *ir.Repository, sources map[int]*source) ([]retrievalUnit, error) {
	units := []retrievalUnit{}
	for file, f := range r.Files {
		src := sources[file]
		if src == nil || len(src.data) == 0 {
			continue
		}
		add := func(kind string, start, end int) int {
			id := "u:" + hashBytes([]byte(fmt.Sprintf("%s:%s:%s:%d:%d", src.path, src.hash, kind, start, end)))[:24]
			units = append(units, retrievalUnit{
				Unit: Unit{ID: id, Kind: kind, File: src.path, Extent: src.span(start, end), Evidence: []string{}},
				file: file, start: start, end: end,
			})
			return len(units) - 1
		}
		first := len(units)
		add("document", 0, len(src.data))
		if f.Lang != ir.LangMarkdown {
			// Bounded physical source blocks cover literals/comments outside symbols
			// as well as unrecognized declarations. Never infer language semantics.
			for start := 0; start < len(src.data); {
				_, end := src.excerpt(start, len(src.data), 2048)
				if end <= start {
					return nil, fmt.Errorf("source block made no progress")
				}
				add("source_block", start, end)
				start = end
			}
		} else {
			type heading struct{ start, end, level int }
			headings := []heading{}
			for _, n := range f.Nodes {
				kind := r.String(n.Kind)
				unitKind := ""
				switch kind {
				case "paragraph":
					unitKind = "paragraph"
				case "list_item":
					unitKind = "list_item"
				case "pipe_table":
					unitKind = "table"
				case "fenced_code_block", "indented_code_block":
					unitKind = "code_block"
				case "atx_heading", "setext_heading":
					unitKind = "heading"
				}
				if unitKind == "" {
					continue
				}
				a, z, err := src.offsets(n.Span)
				if err != nil {
					return nil, fmt.Errorf("retrieval unit in %s: %w", src.path, err)
				}
				if unitKind == "heading" {
					level := 0
					if kind == "atx_heading" {
						text := strings.TrimLeft(string(src.data[a:z]), " \t")
						level = len(text) - len(strings.TrimLeft(text, "#"))
					} else {
						level = 2
						for _, child := range n.Children {
							if r.String(f.Nodes[child].Kind) == "setext_h1_underline" {
								level = 1
							}
						}
					}
					headings = append(headings, heading{start: a, end: z, level: level})
					continue
				}
				if unitKind == "list_item" {
					for _, child := range n.Children {
						marker := r.String(f.Nodes[child].Kind)
						if marker == "task_list_marker_checked" || marker == "task_list_marker_unchecked" {
							unitKind = "checklist_item"
						}
					}
				}
				add(unitKind, a, z)
			}
			sort.Slice(headings, func(i, j int) bool { return headings[i].start < headings[j].start })
			ends := make([]int, len(headings))
			stack := []int{}
			for i, h := range headings {
				for len(stack) > 0 && headings[stack[len(stack)-1]].level >= h.level {
					ends[stack[len(stack)-1]] = h.start
					stack = stack[:len(stack)-1]
				}
				ends[i] = len(src.data)
				stack = append(stack, i)
			}
			for i, h := range headings {
				index := add("section", h.start, ends[i])
				sp := src.span(h.start, h.end)
				units[index].Heading = &sp
				content := src.span(h.end, ends[i])
				units[index].Content = &content
			}
		}
		assignUnitParents(units[first:])
	}
	return units, nil
}

func assignUnitParents(units []retrievalUnit) {
	order := make([]int, len(units))
	for i := range order {
		order[i] = i
	}
	priority := func(kind string) int {
		switch kind {
		case "document":
			return 0
		case "section":
			return 1
		case "list_item", "checklist_item":
			return 2
		}
		return 3
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := units[order[i]], units[order[j]]
		if a.start != b.start {
			return a.start < b.start
		}
		if a.end != b.end {
			return a.end > b.end
		}
		if priority(a.Kind) != priority(b.Kind) {
			return priority(a.Kind) < priority(b.Kind)
		}
		return a.ID < b.ID
	})
	stack := []int{}
	for _, i := range order {
		for len(stack) > 0 {
			p := units[stack[len(stack)-1]]
			if p.start <= units[i].start && p.end >= units[i].end {
				break
			}
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			units[i].Parent = units[stack[len(stack)-1]].ID
		}
		stack = append(stack, i)
	}
}

func rankUnit(u retrievalUnit, src *source, query string) Reason {
	words := terms(query)
	body := strings.ToLower(string(src.data[u.start:u.end]))
	path := strings.ToLower(src.path)
	bodyScore, pathScore := 0, 0
	for _, word := range words {
		if strings.Contains(body, word) {
			bodyScore += 30
		}
		if strings.Contains(path, word) {
			pathScore += 5
		}
	}
	fields := []string{}
	if bodyScore > 0 {
		fields = append(fields, "body")
	}
	if pathScore > 0 {
		fields = append(fields, "path")
	}
	return Reason{Strategy: "lexical_unit", Score: bodyScore + pathScore,
		MatchedFields: fields, Components: map[string]int{"body": bodyScore, "path": pathScore}}
}

func includeUnit(b *Bundle, c candidate, src *source, o Options) (*Bundle, bool, error) {
	u := c.unit
	for mode := 0; mode < 5; mode++ {
		a, z := u.start, u.end
		entry := u.Unit
		entry.Reason, entry.Completeness = c.reason, "full_unit"
		if mode > 0 {
			a, z = src.queryExcerpt(a, z, b.Query, 900>>(mode-1))
			if a == u.start && z == u.end {
				continue
			}
			entry.Completeness = "unit_excerpt"
		}
		trial := clone(b)
		entry.Evidence = []string{addEvidence(trial, src, a, z, "source")}
		trial.Units = append(trial.Units, entry)
		if mode > 0 {
			trial.Omissions.Excerpts++
		}
		ok, err := fits(trial, o, true)
		if err != nil {
			return b, false, err
		}
		if ok {
			return trial, true, nil
		}
	}
	return b, false, nil
}

func choose(ctx context.Context, r *ir.Repository, o Options, sources map[int]*source) ([]candidate, []string, bool, error) {
	candidates, ids, limited, err := chooseSymbols(ctx, r, o, sources)
	if err != nil {
		return nil, nil, false, err
	}
	units := []retrievalUnit{}
	if len(o.Units) > 0 || len(o.Symbols) == 0 {
		units, err = retrievalUnits(r, sources)
		if err != nil {
			return nil, nil, false, err
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, false, err
		}
	}
	ranked := []candidate{}
	explicit := len(o.Symbols)+len(o.Units) > 0
	byID := map[string]*retrievalUnit{}
	for i := range units {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, nil, false, err
			}
		}
		byID[units[i].ID] = &units[i]
	}
	seen := map[string]bool{}
	for _, id := range o.Units {
		u, ok := byID[id]
		if !ok {
			return nil, nil, false, fmt.Errorf("requested unit not found in permitted snapshot: %q", id)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ranked = append(ranked, candidate{node: -1, unit: u, seed: true, reason: Reason{Strategy: "explicit_unit", Score: 100000}})
		ids = append(ids, id)
	}
	if !explicit {
		for i := range units {
			u := &units[i]
			reason := rankUnit(*u, sources[u.file], o.Query)
			if reason.Score == 0 {
				continue
			}
			ranked = append(ranked, candidate{node: -1, unit: u, reason: reason})
		}
		sort.SliceStable(ranked, func(i, j int) bool {
			a, b := ranked[i], ranked[j]
			if a.reason.Score != b.reason.Score {
				return a.reason.Score > b.reason.Score
			}
			// Prefer a compact complete block over a large containing document.
			as, bs := a.unit.end-a.unit.start, b.unit.end-b.unit.start
			if as != bs {
				return as < bs
			}
			return a.unit.ID < b.unit.ID
		})
		kept := []candidate{}
		for _, c := range ranked {
			covered := false
			for _, prev := range kept {
				if prev.unit.file == c.unit.file && prev.unit.start <= c.unit.start && prev.unit.end >= c.unit.end {
					covered = true
					break
				}
			}
			if !covered {
				kept = append(kept, c)
			}
			if len(kept) == o.MaxCandidates {
				limited = true
				break
			}
		}
		ranked = kept
		// Lexical candidates may be omitted, but the highest ranked seed must
		// survive. Explicit symbol behavior remains unchanged.
		for i := range candidates {
			candidates[i].seed = false
		}
		ids = []string{}
	}
	if len(ranked) > o.MaxUnits && explicit {
		return nil, nil, false, fmt.Errorf("unit seeds exceed unit limit")
	}
	combined := append(candidates, ranked...)
	if !explicit {
		sort.SliceStable(combined, func(i, j int) bool { return combined[i].reason.Score > combined[j].reason.Score })
		if len(combined) > 0 {
			combined[0].seed = true
			id := ""
			if combined[0].unit != nil {
				id = combined[0].unit.ID
			} else {
				id = r.String(r.Symbols[combined[0].node].ID)
			}
			ids = append(ids, id)
		}
	}
	if len(combined) == 0 {
		return nil, nil, false, fmt.Errorf("no matching symbols or retrieval units in permitted indexed source")
	}
	if explicit && len(combined) > o.MaxCandidates {
		return nil, nil, false, fmt.Errorf("seed count exceeds candidate limit")
	}
	if len(combined) > o.MaxCandidates {
		combined = combined[:o.MaxCandidates]
		limited = true
	}
	return combined, ids, limited, nil
}
