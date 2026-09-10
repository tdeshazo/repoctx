package agentctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestImportOccurrencesCoverRepositoryAndExternalModules(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test\n\ngo 1.23\n")
	write(t, root, "main.go", `package main

import (
	"example.test/sub"
	"fmt"
)

func Run() string { fmt.Println(sub.Answer()); return "ok" }
`)
	write(t, root, "sub/sub.go", "package sub\n\nfunc Answer() string { return \"answer\" }\n")

	repo, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if got := countEdges(repo, ir.EdgeImports, "example.test/sub"); got != 1 {
		t.Fatalf("repository-module import occurrences=%d, want 1", got)
	}
	if got := countEdges(repo, ir.EdgeImports, "fmt"); got != 1 {
		t.Fatalf("external import occurrences=%d, want 1", got)
	}

	// Module-level Go imports are unit-owned. Verify their graph arcs retain
	// both occurrence sites, which is the source location consumed by context
	// evidence and relationship rendering.
	for _, raw := range []string{"example.test/sub", "fmt"} {
		for _, edge := range repo.Edges {
			if edge.Kind != ir.EdgeImports || repo.String(edge.Text) != raw {
				continue
			}
			matched := false
			for _, target := range importTargets(repo, edge) {
				if target < len(repo.Symbols) || !importArcTargets(repo, edge, target) {
					continue
				}
				x := repo.Graph.External[target-len(repo.Symbols)]
				label := repo.String(x.Name)
				if label == "module:"+raw || importUnitMatches(repo, edge, label, raw) {
					matched = true
				}
			}
			if !matched {
				t.Fatalf("import %q did not retain an external/unit graph target", raw)
			}
		}
	}

	// Package-level Go imports are owned by the compilation unit. Build starts
	// relationship rendering from selected symbols, so its public bundle does
	// not currently expose those unit-owned import arcs.
	runID := ""
	for _, s := range repo.Symbols {
		if repo.String(s.Name) == "Run" {
			runID = repo.String(s.ID)
			break
		}
	}
	if runID == "" {
		t.Fatal("missing Go Run symbol")
	}
	result, err := Build(repo, Options{Root: root, Symbols: []string{runID}, Relations: []ir.EdgeKind{ir.EdgeImports}, MaxBytes: 16000, MaxRelations: 16})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range result.Bundle.Relationships {
		if rel.Kind == "imports" {
			t.Fatalf("unit-owned Go imports unexpectedly rendered publicly: %#v", rel)
		}
	}

	// Python imports inside a function exercise the relationship-site path,
	// including one repository module and one external module.
	write(t, root, "pkg/util.py", "def helper():\n    return 1\n")
	write(t, root, "pkg/main.py", "def run():\n    from pkg.util import helper\n    import external_library\n    return helper()\n")
	repo, err = compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, s := range repo.Symbols {
		if repo.String(s.Name) == "run" {
			id = repo.String(s.ID)
			break
		}
	}
	if id == "" {
		t.Fatal("missing Python run symbol")
	}
	result, err = Build(repo, Options{Root: root, Symbols: []string{id}, Relations: []ir.EdgeKind{ir.EdgeImports}, MaxBytes: 16000, MaxRelations: 16})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, rel := range result.Bundle.Relationships {
		if rel.Kind == "imports" && len(rel.Sites) > 0 {
			seen[rel.To] = true
		}
	}
	if !seen["unit:py:pkg.util"] || !seen["module:external_library"] {
		t.Fatalf("import relationship sites missing: seen=%v rels=%#v", seen, result.Bundle.Relationships)
	}
	for _, ev := range result.Bundle.Evidence {
		if ev.Role == "imports" && !strings.Contains(ev.Text, "external_library") {
			continue
		}
		if ev.Role == "imports" && !strings.Contains(ev.Text, "pkg.util") {
			t.Fatalf("unexpected import evidence: %q", ev.Text)
		}
	}
}

func countEdges(repo *ir.Repository, kind ir.EdgeKind, text string) int {
	n := 0
	for _, edge := range repo.Edges {
		if edge.Kind == kind && repo.String(edge.Text) == text {
			n++
		}
	}
	return n
}

func importTargets(repo *ir.Repository, edge ir.Edge) []int {
	from := graphUnitForFile(repo, edge.From.File)
	if edge.OwnerSymbol > 0 {
		from = edge.OwnerSymbol - 1
	}
	if from < 0 || from+1 >= len(repo.Graph.Out.Offsets) {
		return nil
	}
	var targets []int
	for i := repo.Graph.Out.Offsets[from]; i < repo.Graph.Out.Offsets[from+1]; i++ {
		if repo.Graph.Out.Kinds[i] == ir.EdgeImports {
			targets = append(targets, int(repo.Graph.Out.Targets[i]))
		}
	}
	return targets
}

// Keep os imported in this focused test even when a future compiler changes
// the fixture helper implementation to use direct writes.
var _ = os.FileMode(0)

func TestGoModIsOutsideM0FreshnessScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module old.example\n\ngo 1.23\n")
	write(t, root, "main.go", "package main\nfunc Answer() {}\n")
	repo, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module changed.example\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}
	var id string
	for _, s := range repo.Symbols {
		if repo.String(s.Name) == "Answer" {
			id = repo.String(s.ID)
		}
	}
	if id == "" {
		t.Fatal("missing Answer symbol")
	}
	if _, err := Build(repo, Options{Root: root, Symbols: []string{id}, MaxBytes: 12000}); err != nil {
		t.Fatalf("go.mod change should remain outside M0 freshness checks: %v", err)
	}
}

func TestGoModDenyPrefixDoesNotBecomeAnUnstatedFreshnessGuarantee(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module old.example\n\ngo 1.23\n")
	write(t, root, "main.go", "package main\nfunc Answer() {}\n")
	// M0 records the current boundary: auxiliary module metadata is read for
	// graph construction even when the source allow/deny scope excludes it.
	// M2 must make this input caller-scoped; this test prevents documentation
	// from accidentally claiming that M0 already does so.
	repo, err := compiler.Compile(compiler.Options{Root: root, AllowPaths: []string{"main.go"}, DenyPaths: []string{"go.mod"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.Files) != 1 || repo.String(repo.Files[0].Path) != "main.go" {
		t.Fatalf("denied go.mod escaped source inventory: files=%#v", repo.Files)
	}
	if repo.Graph == nil {
		t.Fatal("missing graph for scoped go.mod test")
	}
}
