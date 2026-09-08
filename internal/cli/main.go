// Package cli contains the repoctx command implementation shared by the
// module-root executable and the legacy ./cmd/repoctx entry point.
package cli

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

// Main runs repoctx with args containing the command and its flags. It exits
// the process for command-line usage or execution errors, matching the
// historical cmd/repoctx behavior.
func Main(args []string) {
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "compile":
		compileCmd(args[1:])
	case "stats":
		statsCmd(args[1:])
	case "context":
		contextCmd(args[1:])
	case "validate":
		validateCmd(args[1:])
	case "graph":
		graphCmd(args[1:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `repoctx - compile Go/Python/HTML/CSS/JavaScript/TypeScript/TSX repositories into a compact typed IR

Usage:
  repoctx compile [-root DIR] [-o repo.ir.json.gz] [-pretty]
  repoctx stats REPO_IR
  repoctx graph [-match TEXT|-node ID] [-depth N] REPO_IR
  repoctx context -root DIR -query TEXT [-max-bytes 32768] REPO_IR
  repoctx validate REPO_IR

The IR contains normalized Tree-sitter AST node tables for Python, HTML, CSS,
JavaScript, TypeScript and TSX plus the native Go AST, stable symbols, source-linked occurrence edges,
and a dense forward/reverse CSR symbol graph. Repository symbols occupy graph node
IDs 0..len(symbols)-1; unit/module/unresolved nodes follow them. Tree-sitter
lowering is syntax-only: it does not execute code, type-check, or model embedded
browser languages.`)
}

func compileCmd(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	root := fs.String("root", ".", "repository root")
	out := fs.String("o", "-", "output path; .gz enables gzip; - writes stdout")
	pretty := fs.Bool("pretty", false, "pretty-print JSON")
	python := fs.String("python", "", "deprecated compatibility flag; Python uses native Tree-sitter")
	maxBytes := fs.Int64("max-bytes", 2<<20, "maximum source file size")
	var allows, denies repeated
	fs.Var(&allows, "allow", "permitted source file/directory prefix before indexing; repeat")
	fs.Var(&denies, "deny", "denied source file/directory prefix before indexing; repeat")
	_ = fs.Parse(args)
	repo, err := compiler.Compile(compiler.Options{Root: *root, Python: *python, MaxBytes: *maxBytes, AllowPaths: allows, DenyPaths: denies})
	if err != nil {
		fatal(err)
	}
	if err := compiler.Write(repo, *out, *pretty); err != nil {
		fatal(err)
	}
}

func statsCmd(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: repoctx stats REPO_IR")
		os.Exit(2)
	}
	repo, err := compiler.Read(args[0])
	if err != nil {
		fatal(err)
	}
	langs := map[ir.Language]int{}
	nodes := 0
	for _, f := range repo.Files {
		langs[f.Lang]++
		nodes += len(f.Nodes)
	}
	graphNodes, graphArcs := 0, 0
	if repo.Graph != nil {
		graphNodes = len(repo.Symbols) + len(repo.Graph.External)
		graphArcs = len(repo.Graph.Out.Targets)
	}
	fmt.Printf("IR: %s\nfiles: %d (go=%d python=%d html=%d css=%d javascript=%d typescript=%d tsx=%d)\nnodes: %d\nsymbols: %d\noccurrence edges: %d\ngraph nodes: %d\ngraph arcs: %d\ndiagnostics: %d\nstrings: %d\n", repo.Version, len(repo.Files), langs[ir.LangGo], langs[ir.LangPython], langs[ir.LangHTML], langs[ir.LangCSS], langs[ir.LangJavaScript], langs[ir.LangTypeScript], langs[ir.LangTSX], nodes, len(repo.Symbols), len(repo.Edges), graphNodes, graphArcs, len(repo.Diagnostics), len(repo.Strings))
}

func graphCmd(args []string) {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	match := fs.String("match", "", "case-insensitive symbol/external substring")
	node := fs.Int("node", -1, "dense graph node ID")
	depth := fs.Int("depth", 1, "neighborhood depth")
	limit := fs.Int("limit", 50, "maximum printed graph nodes")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: repoctx graph [-match TEXT|-node ID] [-depth N] REPO_IR")
		os.Exit(2)
	}
	repo, err := compiler.Read(fs.Arg(0))
	if err != nil {
		fatal(err)
	}
	if repo.Graph == nil {
		fatal(fmt.Errorf("IR has no symbol graph"))
	}
	starts := []uint32{}
	if *node >= 0 {
		starts = append(starts, uint32(*node))
	} else {
		needle := strings.ToLower(*match)
		for i := 0; i < graphNodeCount(repo); i++ {
			label := strings.ToLower(graphNodeLabel(repo, uint32(i)))
			if needle == "" || strings.Contains(label, needle) {
				starts = append(starts, uint32(i))
			}
			if len(starts) >= *limit {
				break
			}
		}
	}
	if len(starts) == 0 {
		fmt.Println("no matching graph nodes")
		return
	}
	seen := map[uint32]int{}
	queue := append([]uint32(nil), starts...)
	for _, n := range starts {
		seen[n] = 0
	}
	for len(queue) > 0 && len(seen) < *limit {
		n := queue[0]
		queue = queue[1:]
		d := seen[n]
		if d >= *depth {
			continue
		}
		for _, csr := range []*ir.CSR{&repo.Graph.Out, &repo.Graph.In} {
			for _, nb := range csrNeighbors(csr, n) {
				if _, ok := seen[nb]; !ok && len(seen) < *limit {
					seen[nb] = d + 1
					queue = append(queue, nb)
				}
			}
		}
	}
	ids := make([]int, 0, len(seen))
	for n := range seen {
		ids = append(ids, int(n))
	}
	sort.Ints(ids)
	for _, ni := range ids {
		n := uint32(ni)
		fmt.Printf("[%d] %s\n", n, graphNodeLabel(repo, n))
		printCSR(repo, "  ->", n, &repo.Graph.Out)
		printCSR(repo, "  <-", n, &repo.Graph.In)
	}
}

func csrNeighbors(c *ir.CSR, node uint32) []uint32 {
	if int(node)+1 >= len(c.Offsets) {
		return nil
	}
	start, end := c.Offsets[node], c.Offsets[node+1]
	out := make([]uint32, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, c.Targets[i])
	}
	return out
}

func printCSR(repo *ir.Repository, prefix string, node uint32, c *ir.CSR) {
	if int(node)+1 >= len(c.Offsets) {
		return
	}
	start, end := c.Offsets[node], c.Offsets[node+1]
	for i := start; i < end; i++ {
		w := c.Weights[i]
		ws := ""
		if w > 1 {
			ws = " x" + strconv.FormatUint(uint64(w), 10)
		}
		fmt.Printf("%s %-10s [%d] %s%s\n", prefix, edgeKindName(c.Kinds[i]), c.Targets[i], graphNodeLabel(repo, c.Targets[i]), ws)
	}
}

func graphNodeCount(repo *ir.Repository) int {
	if repo.Graph == nil {
		return len(repo.Symbols)
	}
	return len(repo.Symbols) + len(repo.Graph.External)
}

func graphNodeLabel(repo *ir.Repository, node uint32) string {
	if int(node) < len(repo.Symbols) {
		s := repo.Symbols[node]
		return str(repo, s.ID) + " (" + symbolKindName(s.Kind) + ")"
	}
	i := int(node) - len(repo.Symbols)
	if repo.Graph == nil || i < 0 || i >= len(repo.Graph.External) {
		return "<invalid>"
	}
	x := repo.Graph.External[i]
	return str(repo, x.Name) + " (" + externalKindName(x.Kind) + ")"
}

func str(repo *ir.Repository, ref int) string {
	if ref <= 0 || ref > len(repo.Strings) {
		return ""
	}
	return repo.Strings[ref-1]
}

func edgeKindName(k ir.EdgeKind) string {
	return map[ir.EdgeKind]string{ir.EdgeDefines: "defines", ir.EdgeImports: "imports", ir.EdgeCalls: "calls", ir.EdgeReferences: "references"}[k]
}

func symbolKindName(k ir.SymbolKind) string {
	return map[ir.SymbolKind]string{ir.SymModule: "module", ir.SymType: "type", ir.SymFunction: "function", ir.SymMethod: "method", ir.SymVariable: "variable", ir.SymConstant: "constant"}[k]
}

func externalKindName(k ir.GraphExternalKind) string {
	return map[ir.GraphExternalKind]string{ir.GraphExternalUnit: "unit", ir.GraphExternalModule: "module", ir.GraphExternalSymbol: "unresolved"}[k]
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "repoctx:", err); os.Exit(1) }
