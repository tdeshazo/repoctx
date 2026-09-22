package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/tdeshazo/repoctx/internal/lang/goast"
	"github.com/tdeshazo/repoctx/internal/lang/pyast"
	"github.com/tdeshazo/repoctx/internal/lang/treeast"
	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
	repomanifest "github.com/tdeshazo/repoctx/pkg/manifest"
)

const IRVersion = "repoctx.ir/v1alpha5"

type Options struct {
	Root               string
	Python             string // retained for API compatibility; Python uses native Tree-sitter
	CacheDir           string // optional trusted, caller-owned parse-fragment cache outside Root
	MaxBytes           int64
	MaxReadBytes       int64 // total bytes across both verification passes; default 256 MiB
	MaxEntries         int   // total discovery entries per pass; default 100000
	IgnoreDirs         map[string]bool
	AllowPaths         []string // optional source/auxiliary file or directory prefixes; not globs
	DenyPaths          []string // evaluated before parsing/indexing
	Manifest           string   // optional repository-relative canonical manifest
	RepositoryRevision string   // optional caller-supplied full Git object ID
	RepositoryDirty    *bool    // optional caller-supplied worktree state
}

type sourceFile struct {
	rel  string
	lang ir.Language
}

func Compile(opts Options) (*ir.Repository, error) {
	repo, _, err := compile(opts)
	return repo, err
}

// CompileWithStats compiles identically to Compile and reports optional
// content-addressed parse-fragment cache use.
func CompileWithStats(opts Options) (*ir.Repository, CacheStats, error) {
	return compile(opts)
}

func compile(opts Options) (*ir.Repository, CacheStats, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, CacheStats{}, err
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 2 << 20
	}
	if opts.MaxReadBytes == 0 {
		opts.MaxReadBytes = 256 << 20
	}
	if opts.MaxEntries == 0 {
		opts.MaxEntries = 100000
	}
	badFileLimit := opts.MaxBytes < 1 || opts.MaxBytes > 16<<20
	badReadLimit := opts.MaxReadBytes < 1 || opts.MaxReadBytes > 256<<20
	badEntryLimit := opts.MaxEntries < 1 || opts.MaxEntries > 100000
	if badFileLimit || badReadLimit || badEntryLimit {
		return nil, CacheStats{}, fmt.Errorf("invalid compilation resource limits")
	}
	if opts.IgnoreDirs == nil {
		opts.IgnoreDirs = defaultIgnoreDirs()
	}
	repository, err := repositoryIdentity(opts)
	if err != nil {
		return nil, CacheStats{}, err
	}

	for _, group := range [][]string{opts.AllowPaths, opts.DenyPaths} {
		for _, p := range group {
			q := strings.TrimSuffix(p, "/")
			if !fs.ValidPath(q) || q == "." || strings.ContainsAny(q, "\\\x00:*") {
				return nil, CacheStats{}, fmt.Errorf("invalid path prefix %q", p)
			}
		}
	}
	sourceRoot, err := sourceroot.Open(root)
	if err != nil {
		return nil, CacheStats{}, err
	}
	defer sourceRoot.Close()
	profile := compilationProfile(opts)
	remaining := opts.MaxReadBytes
	manifest, contents, err := captureInputs(sourceRoot, profile, &remaining)
	if err != nil {
		return nil, CacheStats{}, err
	}
	manifest.Repository = repository
	profileID, err := manifest.ProfileID()
	if err != nil {
		return nil, CacheStats{}, err
	}
	cache, err := newParseCache(root, opts.CacheDir, profileID)
	if err != nil {
		return nil, CacheStats{}, err
	}
	var repo *ir.Repository
	var st *ir.Strings
	if cache == nil {
		repo, st = parseInputs(opts, manifest, contents)
	} else {
		repo, st, err = parseInputsCached(opts, manifest, contents, cache)
		if err != nil {
			return nil, CacheStats{}, err
		}
	}
	linkRepository(repo, st, goModule(contents["go.mod"]))
	if opts.Manifest != "" {
		entities, err := repomanifest.CompileEntities(
			root,
			opts.Manifest,
			repomanifest.Limits{},
			func(path string) bool { return pathAllowed(path, opts) },
		)
		if err != nil {
			return nil, CacheStats{}, fmt.Errorf("compile repository entities: %w", err)
		}
		repo.Entities = entities
	}
	if err := repo.Validate(); err != nil {
		return nil, CacheStats{}, fmt.Errorf("compiler invariant: %w", err)
	}
	if err := verifyInputs(sourceRoot, manifest, &remaining); err != nil {
		return nil, CacheStats{}, err
	}
	stats := CacheStats{}
	if cache != nil {
		stats = cache.stats
	}
	return repo, stats, nil
}

func parseInputs(opts Options, manifest *ir.InputManifest, contents map[string][]byte) (*ir.Repository, *ir.Strings) {
	st := ir.NewStrings()
	repo := &ir.Repository{Version: IRVersion, Root: st.Intern("."), Files: []ir.File{}, Inputs: manifest}
	for _, input := range manifest.Sources {
		sf := sourceFile{rel: input.Path, lang: sourceLanguage(input.Path)}
		data := contents[input.Path]
		fidx := len(repo.Files)
		file := ir.File{Path: st.Intern(sf.rel), Lang: sf.lang, Hash: input.SHA256}
		switch sf.lang {
		case ir.LangPython:
			file.Unit = st.Intern(pythonModule(sf.rel))
		case ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX, ir.LangMarkdown:
			file.Unit = st.Intern(strings.TrimSuffix(sf.rel, filepath.Ext(sf.rel)))
		}
		appendDiagnostic := func(err error) {
			repo.Diagnostics = append(repo.Diagnostics, ir.Diagnostic{
				Severity: ir.SeverityError, File: fidx + 1, Message: st.Intern(err.Error()),
			})
		}
		appendSymbols := func(symbols []treeast.LocalSymbol) {
			base := len(repo.Symbols)
			for _, symbol := range symbols {
				parent := symbol.Parent
				if parent > 0 {
					parent += base
				}
				repo.Symbols = append(repo.Symbols, ir.Symbol{
					Name: st.Intern(symbol.Name), Kind: symbol.Kind, File: fidx,
					Node: symbol.Node, Parent: parent,
				})
			}
		}
		appendEdges := func(edges []treeast.LocalEdge) {
			for _, edge := range edges {
				repo.Edges = append(repo.Edges, ir.Edge{
					Kind: edge.Kind, From: ir.Ref{File: fidx, Node: edge.Node},
					Text: st.Intern(edge.Text),
				})
			}
		}
		switch sf.lang {
		case ir.LangGo:
			result, err := goast.Parse(data, st)
			if err != nil {
				appendDiagnostic(err)
				break
			}
			file.Nodes, file.Roots, file.Unit = result.Nodes, result.Roots, st.Intern(result.Unit)
			base := len(repo.Symbols)
			for _, symbol := range result.Symbols {
				parent := symbol.Parent
				if parent > 0 {
					parent += base
				}
				repo.Symbols = append(repo.Symbols, ir.Symbol{
					Name: st.Intern(symbol.Name), Kind: symbol.Kind, File: fidx,
					Node: symbol.Node, Parent: parent, Receiver: st.Intern(symbol.Receiver),
				})
			}
			for _, edge := range result.Edges {
				repo.Edges = append(repo.Edges, ir.Edge{
					Kind: edge.Kind, From: ir.Ref{File: fidx, Node: edge.Node},
					Text: st.Intern(edge.Text),
				})
			}
		case ir.LangPython:
			result, err := pyast.Parse(opts.Python, data, st)
			if err != nil {
				appendDiagnostic(err)
				break
			}
			file.Nodes, file.Roots = result.Nodes, result.Roots
			appendSymbols(result.Symbols)
			appendEdges(result.Edges)
		case ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX, ir.LangMarkdown:
			result, err := treeast.Parse(treeLanguage(sf.lang), data, st)
			if err != nil {
				appendDiagnostic(err)
				break
			}
			file.Nodes, file.Roots = result.Nodes, result.Roots
			appendSymbols(result.Symbols)
			appendEdges(result.Edges)
		}
		repo.Files = append(repo.Files, file)
	}
	return repo, st
}

func parseInputsCached(opts Options, manifest *ir.InputManifest, contents map[string][]byte, cache *parseCache) (*ir.Repository, *ir.Strings, error) {
	st := ir.NewStrings()
	repo := &ir.Repository{Version: IRVersion, Root: st.Intern("."), Files: []ir.File{}, Inputs: manifest}

	for _, input := range manifest.Sources {
		sf := sourceFile{rel: input.Path, lang: sourceLanguage(input.Path)}
		data := contents[input.Path]
		var fragment parseFragment
		hit := false
		var err error
		if cache != nil {
			fragment, hit, err = cache.load(sf.lang, input.SHA256)
			if err != nil {
				return nil, nil, err
			}
		}
		if !hit {
			fragment = parseSource(opts, sf.lang, data, input.SHA256)
			if cache != nil {
				if err := cache.store(fragment); err != nil {
					return nil, nil, err
				}
			}
		}
		materializeFragment(repo, st, sf.rel, fragment)
	}
	return repo, st, nil
}

func parseSource(opts Options, language ir.Language, data []byte, sourceSHA string) parseFragment {
	st := ir.NewStrings()
	fragment := parseFragment{Version: parseFragmentVersion, SourceSHA: sourceSHA,
		Language: language, File: ir.File{Lang: language, Hash: sourceSHA},
		Symbols: []ir.Symbol{}, Edges: []ir.Edge{}, Diagnostics: []ir.Diagnostic{}}
	appendDiagnostic := func(err error) {
		fragment.Diagnostics = append(fragment.Diagnostics, ir.Diagnostic{
			Severity: ir.SeverityError, File: 1, Message: st.Intern(err.Error()),
		})
	}
	appendSymbols := func(symbols []treeast.LocalSymbol) {
		for _, symbol := range symbols {
			fragment.Symbols = append(fragment.Symbols, ir.Symbol{
				Name: st.Intern(symbol.Name), Kind: symbol.Kind, Node: symbol.Node,
				Parent: symbol.Parent,
			})
		}
	}
	appendEdges := func(edges []treeast.LocalEdge) {
		for _, edge := range edges {
			fragment.Edges = append(fragment.Edges, ir.Edge{
				Kind: edge.Kind, From: ir.Ref{Node: edge.Node}, Text: st.Intern(edge.Text),
			})
		}
	}

	switch language {
	case ir.LangGo:
		result, err := goast.Parse(data, st)
		if err != nil {
			appendDiagnostic(err)
			break
		}
		fragment.File.Nodes, fragment.File.Roots = result.Nodes, result.Roots
		fragment.File.Unit = st.Intern(result.Unit)
		for _, symbol := range result.Symbols {
			fragment.Symbols = append(fragment.Symbols, ir.Symbol{
				Name: st.Intern(symbol.Name), Kind: symbol.Kind, Node: symbol.Node,
				Parent: symbol.Parent, Receiver: st.Intern(symbol.Receiver),
			})
		}
		for _, edge := range result.Edges {
			fragment.Edges = append(fragment.Edges, ir.Edge{
				Kind: edge.Kind, From: ir.Ref{Node: edge.Node}, Text: st.Intern(edge.Text),
			})
		}
	case ir.LangPython:
		result, err := pyast.Parse(opts.Python, data, st)
		if err != nil {
			appendDiagnostic(err)
			break
		}
		fragment.File.Nodes, fragment.File.Roots = result.Nodes, result.Roots
		appendSymbols(result.Symbols)
		appendEdges(result.Edges)
	case ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX, ir.LangMarkdown:
		result, err := treeast.Parse(treeLanguage(language), data, st)
		if err != nil {
			appendDiagnostic(err)
			break
		}
		fragment.File.Nodes, fragment.File.Roots = result.Nodes, result.Roots
		appendSymbols(result.Symbols)
		appendEdges(result.Edges)
	}
	fragment.Strings = st.Values()
	return fragment
}

func materializeFragment(repo *ir.Repository, st *ir.Strings, path string, fragment parseFragment) {
	localString := func(ref int) string {
		if ref <= 0 || ref > len(fragment.Strings) {
			return ""
		}
		return fragment.Strings[ref-1]
	}
	fileIndex := len(repo.Files)
	file := fragment.File
	file.Path = st.Intern(path)
	switch file.Lang {
	case ir.LangPython:
		file.Unit = st.Intern(pythonModule(path))
	case ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX, ir.LangMarkdown:
		// Web-language units remain file-scoped; a renamed fragment must receive
		// the current path rather than retaining a cached generation's unit.
		file.Unit = st.Intern(strings.TrimSuffix(path, filepath.Ext(path)))
	}
	file.Nodes = append([]ir.Node(nil), fragment.File.Nodes...)
	for index := range file.Nodes {
		file.Nodes[index].Kind = st.Intern(localString(file.Nodes[index].Kind))
		file.Nodes[index].Text = st.Intern(localString(file.Nodes[index].Text))
		file.Nodes[index].Children = append([]int(nil), file.Nodes[index].Children...)
	}
	file.Roots = append([]int(nil), fragment.File.Roots...)
	if file.Lang == ir.LangGo {
		file.Unit = st.Intern(localString(fragment.File.Unit))
	}
	base := len(repo.Symbols)
	for _, cached := range fragment.Symbols {
		symbol := cached
		symbol.Name = st.Intern(localString(cached.Name))
		symbol.Receiver = st.Intern(localString(cached.Receiver))
		symbol.File = fileIndex
		if symbol.Parent > 0 {
			symbol.Parent += base
		}
		repo.Symbols = append(repo.Symbols, symbol)
	}
	for _, cached := range fragment.Edges {
		edge := cached
		edge.From.File = fileIndex
		edge.Text = st.Intern(localString(cached.Text))
		repo.Edges = append(repo.Edges, edge)
	}
	for _, cached := range fragment.Diagnostics {
		diagnostic := cached
		diagnostic.File = fileIndex + 1
		diagnostic.Message = st.Intern(localString(cached.Message))
		repo.Diagnostics = append(repo.Diagnostics, diagnostic)
	}
	repo.Files = append(repo.Files, file)
}

func linkRepository(repo *ir.Repository, st *ir.Strings, goModulePath string) {
	linkParents(repo, st)
	resolveEdges(repo, st)
	assignSymbolIDs(repo, st)
	buildSymbolGraph(repo, st, goModulePath)
	repo.Strings = st.Values()
}

func sourceLanguage(path string) ir.Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return ir.LangGo
	case ".py":
		return ir.LangPython
	case ".html", ".htm":
		return ir.LangHTML
	case ".css":
		return ir.LangCSS
	case ".js", ".mjs", ".cjs":
		return ir.LangJavaScript
	case ".ts", ".mts", ".cts":
		return ir.LangTypeScript
	case ".tsx", ".jsx":
		return ir.LangTSX
	case ".md":
		return ir.LangMarkdown
	default:
		return ir.LangUnknown
	}
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

func buildSymbolGraph(repo *ir.Repository, st *ir.Strings, goModule string) {
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

func goModule(data []byte) string {
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
