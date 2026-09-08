package compiler

import (
	"github.com/tdeshazo/repoctx/pkg/ir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileGoAndPython(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module sample\n\ngo 1.23\n")
	mustWrite(t, filepath.Join(root, "main.go"), `package main
import "fmt"
func ping() { fmt.Println("ping") }
func main() { ping() }
`)
	mustWrite(t, filepath.Join(root, "pkg", "worker.py"), `import pathlib
class Worker:
    def run(self):
        return pathlib.Path(".").exists()
`)
	repo, err := Compile(Options{Root: root, Python: "python3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.Files) != 2 {
		t.Fatalf("files=%d", len(repo.Files))
	}
	if len(repo.Symbols) < 4 {
		t.Fatalf("symbols=%d", len(repo.Symbols))
	}
	var goFile, pyFile bool
	for _, f := range repo.Files {
		if f.Lang == ir.LangGo && f.Unit != 0 {
			goFile = true
		}
		if f.Lang == ir.LangPython && f.Unit != 0 {
			pyFile = true
		}
		if len(f.Nodes) == 0 {
			t.Fatalf("file has no AST nodes")
		}
	}
	if !goFile || !pyFile {
		t.Fatalf("go=%v py=%v", goFile, pyFile)
	}
	for i, s := range repo.Symbols {
		if s.ID == 0 {
			t.Fatalf("symbol %d missing stable ID", i)
		}
	}
	if repo.Graph == nil {
		t.Fatal("missing symbol graph")
	}
	n := len(repo.Symbols) + len(repo.Graph.External)
	if len(repo.Graph.Out.Offsets) != n+1 || len(repo.Graph.In.Offsets) != n+1 {
		t.Fatalf("graph offsets: nodes=%d out=%d in=%d", n, len(repo.Graph.Out.Offsets), len(repo.Graph.In.Offsets))
	}
	if len(repo.Graph.Out.Targets) == 0 {
		t.Fatal("empty graph")
	}
	out := filepath.Join(root, "repo.ir.json.gz")
	if err := Write(repo, out, false); err != nil {
		t.Fatal(err)
	}
	round, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if round.Version != IRVersion || len(round.Files) != 2 {
		t.Fatalf("roundtrip: %#v", round)
	}
}

func TestCompileTreeSitterLanguagesAndMalformedDiagnostics(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "worker.py"), "class Worker:\n    def run(self):\n        return 1\n")
	mustWrite(t, filepath.Join(root, "page.html"), "<main id=\"app\"><h1>Hello</h1></main>")
	mustWrite(t, filepath.Join(root, "theme.css"), ".app { color: red; }")
	mustWrite(t, filepath.Join(root, "worker.js"), "export function run() { return ping(); }\n")
	mustWrite(t, filepath.Join(root, "module.mjs"), "const answer = 42;\n")
	mustWrite(t, filepath.Join(root, "legacy.cjs"), "module.exports = function run() {};\n")
	mustWrite(t, filepath.Join(root, "types.ts"), "export interface Worker { run(): string }\nexport const answer: number = 42;\n")
	mustWrite(t, filepath.Join(root, "module.mts"), "export function run(): number { return 1; }\n")
	mustWrite(t, filepath.Join(root, "common.cts"), "export type Value = string | number;\n")
	mustWrite(t, filepath.Join(root, "component.tsx"), "export function App() { return <main />; }\n")
	mustWrite(t, filepath.Join(root, "legacy.jsx"), "export function Legacy() { return <main />; }\n")
	repo, err := Compile(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.Files) != 11 {
		t.Fatalf("files=%d", len(repo.Files))
	}
	seen := map[ir.Language]bool{}
	for _, f := range repo.Files {
		seen[f.Lang] = true
		if len(f.Nodes) == 0 || len(f.Roots) != 1 {
			t.Fatalf("file %q was not lowered: nodes=%d roots=%v", repo.String(f.Path), len(f.Nodes), f.Roots)
		}
	}
	for _, lang := range []ir.Language{ir.LangPython, ir.LangHTML, ir.LangCSS, ir.LangJavaScript, ir.LangTypeScript, ir.LangTSX} {
		if !seen[lang] {
			t.Fatalf("language %v not discovered", lang)
		}
	}
	if len(repo.Diagnostics) != 0 {
		t.Fatalf("valid Tree-sitter fixture diagnostics=%v", repo.Diagnostics)
	}

	badRoot := t.TempDir()
	mustWrite(t, filepath.Join(badRoot, "broken.py"), "def broken(:\n    pass\n")
	mustWrite(t, filepath.Join(badRoot, "broken.js"), "function broken( {")
	bad, err := Compile(Options{Root: badRoot})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad.Diagnostics) != 2 {
		t.Fatalf("malformed diagnostics=%d", len(bad.Diagnostics))
	}
	for _, d := range bad.Diagnostics {
		if d.Severity != ir.SeverityError || !strings.Contains(bad.String(d.Message), "tree-sitter") {
			t.Fatalf("unexpected malformed diagnostic: %#v", d)
		}
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
