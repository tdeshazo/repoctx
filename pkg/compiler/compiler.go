package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tdeshazo/repoctx/internal/lang/goast"
	"github.com/tdeshazo/repoctx/internal/lang/pyast"
	"github.com/tdeshazo/repoctx/internal/lang/treeast"
	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

const IRVersion = "repoctx.ir/v1alpha3"

type Options struct {
	Root       string
	Python     string // retained for API compatibility; Python uses native Tree-sitter
	MaxBytes   int64
	IgnoreDirs map[string]bool
	AllowPaths []string // optional source file/directory prefixes; not globs
	DenyPaths  []string // evaluated before parsing/indexing
}

type sourceFile struct {
	abs  string
	rel  string
	lang ir.Language
}

func Compile(opts Options) (*ir.Repository, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 2 << 20
	}
	if opts.IgnoreDirs == nil {
		opts.IgnoreDirs = defaultIgnoreDirs()
	}

	for _, group := range [][]string{opts.AllowPaths, opts.DenyPaths} {
		for _, p := range group {
			q := strings.TrimSuffix(p, "/")
			if !fs.ValidPath(q) || q == "." || strings.ContainsAny(q, "\\\x00:*") {
				return nil, fmt.Errorf("invalid path prefix %q", p)
			}
		}
	}
	sourceRoot, err := sourceroot.Open(root)
	if err != nil {
		return nil, err
	}
	defer sourceRoot.Close()
	st := ir.NewStrings()
	repo := &ir.Repository{Version: IRVersion, Root: st.Intern("."), Files: []ir.File{}}
	files, walkDiags, err := discover(root, opts)
	if err != nil {
		return nil, err
	}
	for _, d := range walkDiags {
		repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityWarning, Message: st.Intern(d)})
	}

	for _, sf := range files {
		data, err := sourceRoot.Read(sf.rel, opts.MaxBytes)
		if err != nil {
			repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityWarning, Message: st.Intern(fmt.Sprintf("read %s: %v", sf.rel, err))})
			continue
		}
		if int64(len(data)) > opts.MaxBytes {
			repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityWarning, Message: st.Intern(fmt.Sprintf("skip %s: %d bytes exceeds max %d", sf.rel, len(data), opts.MaxBytes))})
			continue
		}
		fidx := len(repo.Files)
		f := ir.File{Path: st.Intern(sf.rel), Lang: sf.lang, Hash: fullHash(data)}
		if sf.lang == ir.LangPython {
			f.Unit = st.Intern(pythonModule(sf.rel))
		} else if sf.lang == ir.LangHTML || sf.lang == ir.LangCSS || sf.lang == ir.LangJavaScript || sf.lang == ir.LangTypeScript || sf.lang == ir.LangTSX || sf.lang == ir.LangMarkdown {
			// Web-language units are deliberately file-scoped: cross-file links are
			// unresolved unless a future front end proves the module relationship.
			f.Unit = st.Intern(strings.TrimSuffix(sf.rel, filepath.Ext(sf.rel)))
		}

		switch sf.lang {
		case ir.LangGo:
			res, err := goast.Parse(data, st)
			if err != nil {
				repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityError, File: fidx + 1, Message: st.Intern(err.Error())})
				repo.Files = append(repo.Files, f)
				continue
			}
			f.Nodes, f.Roots, f.Unit = res.Nodes, res.Roots, st.Intern(res.Unit)
			base := len(repo.Symbols)
			for _, s := range res.Symbols {
				parent := 0
				if s.Parent > 0 {
					parent = base + s.Parent
				}
				repo.Symbols = append(repo.Symbols, ir.Symbol{Name: st.Intern(s.Name), Kind: s.Kind, File: fidx, Node: s.Node, Parent: parent, Receiver: st.Intern(s.Receiver)})
			}
			for _, e := range res.Edges {
				repo.Edges = append(repo.Edges, ir.Edge{Kind: e.Kind, From: ir.Ref{File: fidx, Node: e.Node}, Text: st.Intern(e.Text)})
			}
		case ir.LangPython:
			res, err := pyast.Parse(opts.Python, data, st)
			if err != nil {
				repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityError, File: fidx + 1, Message: st.Intern(err.Error())})
				repo.Files = append(repo.Files, f)
				continue
			}
			f.Nodes, f.Roots = res.Nodes, res.Roots
			base := len(repo.Symbols)
			for _, s := range res.Symbols {
				parent := 0
				if s.Parent > 0 {
					parent = base + s.Parent
				}
				repo.Symbols = append(repo.Symbols, ir.Symbol{Name: st.Intern(s.Name), Kind: s.Kind, File: fidx, Node: s.Node, Parent: parent})
			}
			for _, e := range res.Edges {
				repo.Edges = append(repo.Edges, ir.Edge{Kind: e.Kind, From: ir.Ref{File: fidx, Node: e.Node}, Text: st.Intern(e.Text)})
			}
		case ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX, ir.LangMarkdown:
			res, err := treeast.Parse(treeLanguage(sf.lang), data, st)
			if err != nil {
				repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{Severity: ir.SeverityError, File: fidx + 1, Message: st.Intern(err.Error())})
				repo.Files = append(repo.Files, f)
				continue
			}
			f.Nodes, f.Roots = res.Nodes, res.Roots
			base := len(repo.Symbols)
			for _, s := range res.Symbols {
				parent := 0
				if s.Parent > 0 {
					parent = base + s.Parent
				}
				repo.Symbols = append(repo.Symbols, ir.Symbol{Name: st.Intern(s.Name), Kind: s.Kind, File: fidx, Node: s.Node, Parent: parent})
			}
			for _, e := range res.Edges {
				repo.Edges = append(repo.Edges, ir.Edge{Kind: e.Kind, From: ir.Ref{File: fidx, Node: e.Node}, Text: st.Intern(e.Text)})
			}
		}
		repo.Files = append(repo.Files, f)
	}

	linkParents(repo, st)
	resolveEdges(repo, st)
	assignSymbolIDs(repo, st)
	buildSymbolGraph(repo, st, root)
	repo.Strings = st.Values()
	if err := repo.Validate(); err != nil {
		return nil, fmt.Errorf("compiler invariant: %w", err)
	}
	return repo, nil
}

func discover(root string, opts Options) ([]sourceFile, []string, error) {
	var files []sourceFile
	var diags []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, denied := range opts.DenyPaths {
			denied = strings.TrimSuffix(denied, "/")
			if rel == denied || strings.HasPrefix(rel, denied+"/") {
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}
		if walkErr != nil {
			diags = append(diags, fmt.Sprintf("walk %s: %v", path, walkErr))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && opts.IgnoreDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		var lang ir.Language
		switch ext {
		case ".go":
			lang = ir.LangGo
		case ".py":
			lang = ir.LangPython
		case ".html", ".htm":
			lang = ir.LangHTML
		case ".css":
			lang = ir.LangCSS
		case ".js", ".mjs", ".cjs":
			lang = ir.LangJavaScript
		case ".ts", ".mts", ".cts":
			lang = ir.LangTypeScript
		case ".tsx", ".jsx":
			lang = ir.LangTSX
		case ".md":
			lang = ir.LangMarkdown
		default:
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if !pathAllowed(filepath.ToSlash(rel), opts) {
			return nil
		}
		files = append(files, sourceFile{abs: path, rel: filepath.ToSlash(rel), lang: lang})
		return nil
	})
	if err != nil {
		return nil, diags, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, diags, nil
}

func treeLanguage(lang ir.Language) treeast.Language {
	switch lang {
	case ir.LangHTML:
		return treeast.HTML
	case ir.LangCSS:
		return treeast.CSS
	case ir.LangJavaScript:
		return treeast.JavaScript
	case ir.LangTypeScript:
		return treeast.TypeScript
	case ir.LangTSX:
		return treeast.TSX
	case ir.LangMarkdown:
		return treeast.Markdown
	default:
		return treeast.Python
	}
}

func defaultIgnoreDirs() map[string]bool {
	return map[string]bool{
		".git": true, ".hg": true, ".svn": true,
		"vendor": true, "node_modules": true,
		".venv": true, "venv": true, "__pycache__": true,
		"dist": true, "build": true,
	}
}

func pythonModule(rel string) string {
	p := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
	p = strings.TrimSuffix(p, "/__init__")
	p = strings.TrimPrefix(p, "./")
	return strings.ReplaceAll(p, "/", ".")
}

func fullHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func resolveEdges(repo *ir.Repository, st *ir.Strings) {
	// Candidate links only, NOT type-checked dispatch. Restrict by language and
	// unit to avoid linking a Python/library call to an unrelated Go definition.
	values := st.Values()
	byName := map[string][]int{}
	for i, sym := range repo.Symbols {
		if sym.Kind != ir.SymFunction && sym.Kind != ir.SymMethod && sym.Kind != ir.SymType {
			continue
		}
		f := repo.Files[sym.File]
		key := fmt.Sprintf("%d:%s:%s:%s", f.Lang, filepath.Dir(stringAt(values, f.Path)), stringAt(values, f.Unit), stringAt(values, sym.Name))
		byName[key] = append(byName[key], i)
	}
	for i := range repo.Edges {
		e := &repo.Edges[i]
		if e.Kind != ir.EdgeCalls || e.Text == 0 {
			continue
		}
		f := repo.Files[e.From.File]
		raw := stringAt(values, e.Text)
		name := raw
		if p := strings.LastIndex(raw, "."); p >= 0 {
			name = raw[p+1:]
		}
		key := fmt.Sprintf("%d:%s:%s:%s", f.Lang, filepath.Dir(stringAt(values, f.Path)), stringAt(values, f.Unit), name)
		if matches := byName[key]; len(matches) == 1 {
			e.ToSymbol = matches[0] + 1
		}
	}
}

func linkParents(repo *ir.Repository, st *ir.Strings) {
	values := st.Values()
	key := func(fi int, name string) string {
		f := repo.Files[fi]
		return fmt.Sprintf("%d:%s:%s:%s", f.Lang, filepath.Dir(stringAt(values, f.Path)), stringAt(values, f.Unit), name)
	}
	types := map[string][]int{}
	for i, s := range repo.Symbols {
		if s.Kind == ir.SymType {
			k := key(s.File, stringAt(values, s.Name))
			types[k] = append(types[k], i)
		}
	}
	for i := range repo.Symbols {
		s := &repo.Symbols[i]
		if s.Receiver != 0 {
			matches := types[key(s.File, stringAt(values, s.Receiver))]
			if len(matches) == 1 {
				s.Parent = matches[0] + 1
			}
		}
		if repo.Files[s.File].Lang != ir.LangGo || s.Kind == ir.SymMethod || s.Kind == ir.SymFunction {
			continue
		}
		sp := repo.Files[s.File].Nodes[s.Node].Span
		best, size := -1, int(^uint(0)>>1)
		for j, parent := range repo.Symbols {
			if parent.File != s.File || (parent.Kind != ir.SymFunction && parent.Kind != ir.SymMethod) {
				continue
			}
			ps := repo.Files[parent.File].Nodes[parent.Node].Span
			if containsSpan(ps, sp) && spanSize(ps) < size {
				best, size = j, spanSize(ps)
			}
		}
		if best >= 0 {
			s.Parent = best + 1
		}
	}
}

func stringAt(values []string, ref int) string {
	if ref <= 0 || ref > len(values) {
		return ""
	}
	return values[ref-1]
}

func assignSymbolIDs(repo *ir.Repository, st *ir.Strings) {
	values := st.Values()
	base := make([]string, len(repo.Symbols))
	counts := map[string]int{}
	for i := range repo.Symbols {
		base[i] = semanticSymbolBase(repo, i, values)
		counts[base[i]]++
	}
	for i := range repo.Symbols {
		id := base[i]
		if counts[id] > 1 {
			s := repo.Symbols[i]
			if s.File >= 0 && s.File < len(repo.Files) && s.Node >= 0 && s.Node < len(repo.Files[s.File].Nodes) {
				sp := repo.Files[s.File].Nodes[s.Node].Span
				id = fmt.Sprintf("%s@%s:%d:%d:%d", id, stringAt(st.Values(), repo.Files[s.File].Path), sp.SL, sp.SC, i)
			} else {
				id = fmt.Sprintf("%s@%d", id, i)
			}
		}
		repo.Symbols[i].ID = st.Intern(id)
	}
}

func semanticSymbolBase(repo *ir.Repository, idx int, values []string) string {
	s := repo.Symbols[idx]
	lang := "unknown"
	unit := ""
	path := ""
	if s.File >= 0 && s.File < len(repo.Files) {
		f := repo.Files[s.File]
		path = stringAt(values, f.Path)
		unit = stringAt(values, f.Unit)
		switch f.Lang {
		case ir.LangGo:
			lang = "go"
			dir := filepath.ToSlash(filepath.Dir(path))
			if dir == "." {
				dir = ""
			}
			if dir != "" {
				unit = dir + "/" + unit
			}
		case ir.LangPython:
			lang = "py"
		case ir.LangHTML:
			lang = "html"
		case ir.LangCSS:
			lang = "css"
		case ir.LangJavaScript:
			lang = "js"
		case ir.LangTypeScript:
			lang = "ts"
		case ir.LangTSX:
			lang = "tsx"
		case ir.LangMarkdown:
			lang = "md"
		}
	}
	var parts []string
	for cur := idx; cur >= 0; {
		x := repo.Symbols[cur]
		parts = append(parts, stringAt(values, x.Name))
		if x.Parent == 0 && x.Receiver != 0 {
			parts = append(parts, stringAt(values, x.Receiver))
		}
		if x.Parent == 0 {
			break
		}
		cur = x.Parent - 1
		if cur < 0 || cur >= len(repo.Symbols) {
			break
		}
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return fmt.Sprintf("%s:%s#%s", lang, unit, strings.Join(parts, "."))
}

func buildSymbolGraph(repo *ir.Repository, st *ir.Strings, root string) {
	g := &ir.SymbolGraph{}
	values := st.Values()
	external := map[string]uint32{}
	var arcs []ir.GraphArc

	addExternal := func(kind ir.GraphExternalKind, label string) uint32 {
		key := fmt.Sprintf("%d:%s", kind, label)
		if id, ok := external[key]; ok {
			return id
		}
		id := uint32(len(repo.Symbols) + len(g.External))
		g.External = append(g.External, ir.GraphExternal{Name: st.Intern(label), Kind: kind})
		external[key] = id
		return id
	}

	// Build deterministic unit nodes first so top-level definitions and imports
	// have compact source/target anchors. Go import paths use the root go.mod
	// module name when available.
	goModule := readGoModule(root)
	fileUnit := make([]uint32, len(repo.Files))
	unitByImport := map[string]uint32{}
	for fi, f := range repo.Files {
		path := stringAt(values, f.Path)
		unit := stringAt(values, f.Unit)
		var label, importName string
		switch f.Lang {
		case ir.LangGo:
			dir := filepath.ToSlash(filepath.Dir(path))
			if dir == "." {
				dir = ""
			}
			pkgKey := unit
			if dir != "" {
				pkgKey = dir + "/" + unit
			}
			label = "unit:go:" + pkgKey
			if goModule != "" {
				importName = goModule
				if dir != "" {
					importName += "/" + dir
				}
			}
		case ir.LangPython:
			label = "unit:py:" + unit
			importName = unit
		default:
			label = "unit:" + path
		}
		id := addExternal(ir.GraphExternalUnit, label)
		fileUnit[fi] = id
		if importName != "" {
			unitByImport[importName] = id
		}
	}

	// Parent ownership becomes a direct DEFINES relation. Top-level symbols are
	// defined by their file/unit anchor.
	for si, s := range repo.Symbols {
		from := fileUnit[s.File]
		if s.Parent > 0 {
			from = uint32(s.Parent - 1)
		}
		arcs = append(arcs, ir.GraphArc{From: from, To: uint32(si), Kind: ir.EdgeDefines, Weight: 1})
	}

	for ei := range repo.Edges {
		e := &repo.Edges[ei]
		if e.From.File < 0 || e.From.File >= len(repo.Files) {
			continue
		}
		from := fileUnit[e.From.File]
		if owner := owningSymbol(repo, e.From); owner >= 0 {
			from = uint32(owner)
			e.OwnerSymbol = owner + 1
		}
		raw := stringAt(values, e.Text)
		var to uint32
		switch e.Kind {
		case ir.EdgeCalls, ir.EdgeReferences:
			if e.ToSymbol > 0 {
				to = uint32(e.ToSymbol - 1)
			} else {
				to = addExternal(ir.GraphExternalSymbol, "symbol:"+raw)
			}
		case ir.EdgeImports:
			if id, ok := resolveImportedUnit(raw, unitByImport); ok {
				to = id
			} else {
				to = addExternal(ir.GraphExternalModule, "module:"+raw)
			}
		default:
			continue
		}
		arcs = append(arcs, ir.GraphArc{From: from, To: to, Kind: e.Kind, Weight: 1})
	}

	n := len(repo.Symbols) + len(g.External)
	g.Out, g.In = ir.BuildCSR(n, arcs)
	repo.Graph = g
}

func owningSymbol(repo *ir.Repository, ref ir.Ref) int {
	if ref.File < 0 || ref.File >= len(repo.Files) {
		return -1
	}
	f := repo.Files[ref.File]
	if ref.Node < 0 || ref.Node >= len(f.Nodes) {
		return -1
	}
	target := f.Nodes[ref.Node].Span
	best, bestSize := -1, int(^uint(0)>>1)
	for i, s := range repo.Symbols {
		if s.File != ref.File || s.Node < 0 || s.Node >= len(f.Nodes) {
			continue
		}
		sp := f.Nodes[s.Node].Span
		if containsSpan(sp, target) {
			sz := spanSize(sp)
			if sz < bestSize {
				best, bestSize = i, sz
			}
		}
	}
	return best
}

func containsSpan(outer, inner ir.Span) bool {
	startOK := outer.SL < inner.SL || (outer.SL == inner.SL && outer.SC <= inner.SC)
	endOK := outer.EL > inner.EL || (outer.EL == inner.EL && outer.EC >= inner.EC)
	return startOK && endOK
}

func spanSize(s ir.Span) int { return (s.EL-s.SL)*1_000_000 + (s.EC - s.SC) }

func resolveImportedUnit(raw string, units map[string]uint32) (uint32, bool) {
	if id, ok := units[raw]; ok {
		return id, true
	}
	// Python `from pkg.mod import Name` is represented as pkg.mod.Name. Prefer
	// the longest repository module prefix.
	for s := raw; ; {
		p := strings.LastIndex(s, ".")
		if p < 0 {
			break
		}
		s = s[:p]
		if id, ok := units[s]; ok {
			return id, true
		}
	}
	return 0, false
}

func readGoModule(root string) string {
	base, e := sourceroot.Open(root)
	if e != nil {
		return ""
	}
	defer base.Close()
	data, err := base.Read("go.mod", 1<<20)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func pathAllowed(path string, o Options) bool {
	match := func(p string) bool {
		p = strings.TrimSuffix(p, "/")
		return path == p || strings.HasPrefix(path, p+"/")
	}
	for _, p := range o.DenyPaths {
		if match(p) {
			return false
		}
	}
	if len(o.AllowPaths) == 0 {
		return true
	}
	for _, p := range o.AllowPaths {
		if match(p) {
			return true
		}
	}
	return false
}
