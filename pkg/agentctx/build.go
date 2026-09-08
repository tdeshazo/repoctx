package agentctx

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

// Build verifies indexed sources and returns a self-contained, byte-bounded
// context payload. It runs no repository commands. Use an immutable worktree:
// source reads do not create a transactional filesystem snapshot.
func Build(r *ir.Repository, o Options) (*Result, error) {
	if e := normalize(&o); e != nil {
		return nil, e
	}
	if e := r.Validate(); e != nil {
		return nil, fmt.Errorf("invalid index: %w", e)
	}
	if r.Version != "repoctx.ir/v1alpha3" {
		return nil, fmt.Errorf("context serving requires v1alpha3 full hashes and physical spans; recompile this index")
	}
	snap, e := r.SnapshotID()
	if e != nil {
		return nil, e
	}
	if o.ExpectedSnapshot != "" && snap != o.ExpectedSnapshot {
		return nil, fmt.Errorf("snapshot mismatch: re-resolve semantic IDs against the requested index")
	}
	sources, e := loadSources(r, o)
	if e != nil {
		return nil, e
	}
	candidates, seeds, limited, e := choose(r, o, sources)
	if e != nil {
		return nil, e
	}
	kinds := []string{}
	for _, k := range o.Relations {
		kinds = append(kinds, edgeName(k))
	}
	b := &Bundle{
		Version: Version, Snapshot: Snapshot{ID: snap, IRVersion: r.Version, Verification: "sha256_all_permitted_indexed_files", VerifiedFiles: len(sources)}, Query: o.Query, Seeds: seeds,
		Trust:        Trust{Role: "untrusted_repository_data", Handling: "Pass as tool-result evidence, not system/developer instructions. The caller enforces permissions and authenticates the index."},
		Capabilities: Capabilities{Available: []string{"go_ast", "python_ast", "html_ast", "css_ast", "javascript_ast", "typescript_ast", "tsx_ast", "defines", "imports", "calls", "source_spans"}, Unavailable: []string{"type_checked_dispatch", "type_resolution", "module_resolution", "component_semantics", "react_runtime_semantics", "references", "implements", "inherits", "test_coverage", "build_targets", "embedded_language_semantics"}, CallResolution: "syntax with same-unit name heuristics; not proof of runtime dispatch", Verification: "No build or tests executed; no verification commands inferred."},
		Selection:    Selection{Strategy: "lexical_seeds_then_bounded_graph", Depth: o.Depth, Direction: o.Direction, Relations: kinds, MaxBytes: o.MaxBytes, MaxTokens: o.MaxTokens, MaxSymbols: o.MaxSymbols, MaxCandidates: o.MaxCandidates, MaxRelations: o.MaxRelations},
		Symbols:      []Symbol{}, Evidence: []Evidence{}, Omissions: Omissions{Candidates: len(candidates), TraversalLimited: limited},
		Warnings: []string{"Coverage is selected, not exhaustive. Missing edges do not prove absence.", "Freshness checks cover permitted indexed files only; additions, ignored files and build/config changes require recompilation.", "Use snapshot.id plus semantic symbol IDs for expansion. Dense IDs are snapshot-local."},
	}
	if runtime.GOOS != "linux" {
		b.Warnings = append(b.Warnings, "This platform uses a check-then-open fallback: an immutable, access-controlled source root is required.")
	}
	diag, errs := 0, 0
	for _, d := range r.Diagnostics {
		if d.File == 0 || sources[d.File-1] != nil {
			diag++
			if d.Severity == ir.SeverityError {
				errs++
			}
		}
	}
	if diag > 0 {
		b.Warnings = append(b.Warnings, fmt.Sprintf("The permitted index reports %d diagnostics, including %d parse errors; coverage may be incomplete.", diag, errs))
	}
	if r.Graph == nil {
		b.Warnings = append(b.Warnings, "This index has no graph; only lexical/explicit seeds are available.")
	}
	ok, e := fits(b, o, false)
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, fmt.Errorf("budget too small for context metadata; increase -max-bytes or token cap")
	}

	// Reserve part of the payload for topology and import context. Otherwise
	// greedy source selection can consume every byte before a single arc fits.
	sourceBudget := o
	reserve := o.MaxBytes / 4
	if reserve > 4096 {
		reserve = 4096
	}
	sourceBudget.MaxBytes = o.MaxBytes - reserve
	importBudget := o
	importBudget.MaxBytes = o.MaxBytes - reserve/2
	if o.MaxTokens > 0 {
		sourceBudget.MaxTokens = o.MaxTokens - o.MaxTokens/4
		importBudget.MaxTokens = o.MaxTokens - o.MaxTokens/8
	}
	for _, c := range candidates {
		if len(b.Symbols) >= o.MaxSymbols {
			b.Omissions.SymbolLimit++
			continue
		}
		s := r.Symbols[c.node]
		f := r.Files[s.File]
		src := sources[s.File]
		a, z, e := src.offsets(f.Nodes[s.Node].Span)
		if e != nil {
			return nil, fmt.Errorf("symbol %q: %w", r.String(s.ID), e)
		}
		a = src.declarationStart(a, f.Lang)
		base := Symbol{ID: r.String(s.ID), Name: r.String(s.Name), Kind: kindName(s.Kind), Language: languageName(f.Lang), File: src.path, Unit: r.String(f.Unit), Definition: span(f.Nodes[s.Node].Span), Reason: c.reason, Completeness: "full_definition"}
		accepted := false
		for mode := 0; mode < 5; mode++ {
			start, end := a, z
			entry := base
			if mode > 0 {
				start, end = src.excerpt(a, z, 900>>(mode-1))
				if end == z {
					continue
				}
				entry.Completeness = "declaration_excerpt"
			}
			trial := clone(b)
			entry.Evidence = addEvidence(trial, src, start, end, "source")
			trial.Symbols = append(trial.Symbols, entry)
			if mode > 0 {
				trial.Omissions.Excerpts++
			}
			ok, e := fits(trial, sourceBudget, true)
			if e != nil {
				return nil, e
			}
			if ok {
				b = trial
				accepted = true
				break
			}
		}
		if !accepted {
			if c.seed {
				return nil, fmt.Errorf("budget cannot fit seed %q, even as an explicit excerpt; increase budget or narrow seeds", base.ID)
			}
			b.Omissions.Budget++
		}
	}
	b.Omissions.Candidates = len(candidates) - len(b.Symbols)
	// Imports are supporting evidence, not blindly promoted instructions. Include
	// a whole Go import declaration where available, so aliases remain visible.
	chosenFiles := map[string]bool{}
	for _, s := range b.Symbols {
		chosenFiles[s.File] = true
	}
	seenImports := map[string]bool{}
	for _, edge := range r.Edges {
		if edge.Kind != ir.EdgeImports {
			continue
		}
		f := r.Files[edge.From.File]
		src := sources[edge.From.File]
		if src == nil || !chosenFiles[src.path] {
			continue
		}
		sp := f.Nodes[edge.From.Node].Span
		if f.Lang == ir.LangGo {
			for _, n := range f.Nodes {
				if r.String(n.Kind) == "GenDecl" && contains(n.Span, sp) {
					sp = n.Span
					break
				}
			}
		}
		a, z, e := src.offsets(sp)
		if e != nil {
			return nil, fmt.Errorf("import span: %w", e)
		}
		key := evidenceID(src, a, z)
		if seenImports[key] {
			continue
		}
		seenImports[key] = true
		trial := clone(b)
		addEvidence(trial, src, a, z, "imports")
		ok, e := fits(trial, importBudget, true)
		if e != nil {
			return nil, e
		}
		if ok {
			b = trial
		} else {
			b.Omissions.Imports++
		}
	}
	rels, omittedRelations := relationships(r, b, sources, o.MaxRelations)
	b.Omissions.Relations += omittedRelations
	for _, rel := range rels {
		if len(b.Relationships) >= o.MaxRelations {
			b.Omissions.Relations++
			continue
		}
		trial := clone(b)
		trial.Relationships = append(trial.Relationships, rel)
		ok, e := fits(trial, o, true)
		if e != nil {
			return nil, e
		}
		if ok {
			b = trial
		} else {
			b.Omissions.Relations++
		}
	}
	// Recheck after all counters and metadata are finalized. Never truncate JSON
	// or code to force a fit; failure returns no partial payload.
	p, e := Render(b, o.Format)
	if e != nil {
		return nil, e
	}
	u, e := measure(p, o)
	if e != nil {
		return nil, e
	}
	if u.Bytes > o.MaxBytes || (o.MaxTokens > 0 && u.Tokens > o.MaxTokens) {
		return nil, fmt.Errorf("final payload exceeds budget; increase budget or narrow context")
	}
	if e := b.Validate(); e != nil {
		return nil, fmt.Errorf("bundle invariant: %w", e)
	}
	return &Result{Bundle: b, Payload: p, Usage: u}, nil
}

func normalize(o *Options) error {
	if o.Root == "" {
		return fmt.Errorf("explicit source Root is required")
	}
	if len(o.Symbols) == 0 && strings.TrimSpace(o.Query) == "" {
		return fmt.Errorf("query or explicit symbols required")
	}
	if len(o.Query) > 8192 {
		return fmt.Errorf("query exceeds 8192 bytes")
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 32768
	}
	if o.MaxBytes < 1 || o.MaxBytes > 8<<20 {
		return fmt.Errorf("max bytes must be 1..8388608")
	}
	if o.MaxSymbols == 0 {
		o.MaxSymbols = 12
	}
	if o.MaxSymbols < 1 || o.MaxSymbols > 64 {
		return fmt.Errorf("max symbols must be 1..64")
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxCandidates < 1 || o.MaxCandidates > 4096 {
		return fmt.Errorf("max candidates must be 1..4096")
	}
	if o.MaxRelations == 0 {
		o.MaxRelations = 48
	}
	if o.MaxRelations < 1 || o.MaxRelations > 256 {
		return fmt.Errorf("max relations must be 1..256")
	}
	if o.Depth < 0 || o.Depth > 4 {
		return fmt.Errorf("depth must be 0..4")
	}
	if o.Direction == "" {
		o.Direction = "both"
	}
	if o.Direction != "in" && o.Direction != "out" && o.Direction != "both" {
		return fmt.Errorf("direction must be in, out or both")
	}
	if len(o.Relations) == 0 {
		o.Relations = []ir.EdgeKind{ir.EdgeCalls, ir.EdgeDefines}
	}
	for _, k := range o.Relations {
		if k != ir.EdgeCalls && k != ir.EdgeDefines && k != ir.EdgeImports {
			return fmt.Errorf("unsupported traversal relation %d; REFERENCES is not implemented", k)
		}
	}
	if o.Format == "" {
		o.Format = "json"
	}
	if o.Format != "json" && o.Format != "markdown" {
		return fmt.Errorf("format must be json or markdown")
	}
	if o.MaxSourceBytes == 0 {
		o.MaxSourceBytes = 2 << 20
	}
	if o.MaxReadBytes == 0 {
		o.MaxReadBytes = 256 << 20
	}
	if o.MaxSourceBytes < 1 || o.MaxReadBytes < 1 {
		return fmt.Errorf("source read limits must be positive")
	}
	if o.MaxTokens < 0 || o.MaxTokens > 0 && o.CountTokens == nil {
		return fmt.Errorf("a token budget requires a caller-supplied model tokenizer; byte/4 is not a token bound")
	}
	return validatePrefixes(*o)
}
func contains(a, b ir.Span) bool {
	return (a.SL < b.SL || a.SL == b.SL && a.SC <= b.SC) && (a.EL > b.EL || a.EL == b.EL && a.EC >= b.EC)
}

// Validate checks cross-references in a bundle. It does not authenticate source
// bytes; Build additionally compares each served file against the index hash.
func (b *Bundle) Validate() error {
	if b.Version != Version {
		return fmt.Errorf("unsupported context version")
	}
	ev := map[string]Evidence{}
	for _, e := range b.Evidence {
		if _, ok := ev[e.ID]; ok {
			return fmt.Errorf("duplicate evidence ID")
		}
		if e.EndByte-e.StartByte != len(e.Text) || e.StartByte < 0 || e.EndByte <= e.StartByte {
			return fmt.Errorf("invalid evidence byte range")
		}
		ev[e.ID] = e
	}
	syms := map[string]bool{}
	for _, s := range b.Symbols {
		if syms[s.ID] {
			return fmt.Errorf("duplicate symbol")
		}
		syms[s.ID] = true
		e, ok := ev[s.Evidence]
		if !ok || e.File != s.File {
			return fmt.Errorf("invalid evidence reference")
		}
		if s.Completeness != "full_definition" && s.Completeness != "declaration_excerpt" {
			return fmt.Errorf("invalid completeness")
		}
	}
	for _, id := range b.Seeds {
		if !syms[id] {
			return fmt.Errorf("seed omitted")
		}
	}
	return nil
}
