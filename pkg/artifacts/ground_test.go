package artifacts

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestGroundRelationshipsSeparatesProvenance(t *testing.T) {
	repo := groundingFixture(t)
	catalog := minimal()
	owner := "maintainers"
	catalog.Artifacts[0].Owner = &owner
	second := catalog.Artifacts[0]
	second.ID = "a:b"
	second.Owner = nil
	catalog.Artifacts = append(catalog.Artifacts, second)
	catalog.Relationships = append(catalog.Relationships, relationship("a:a", "a:b", "contains"))

	options := GroundOptions{
		DocumentLinks: true,
		GoImports:     &GoImportOptions{GoMod: []byte(groundingGoMod)},
	}
	grounding, err := GroundRelationships(repo, catalog, options)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := GroundRelationships(repo, catalog, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(grounding, repeated) {
		t.Fatal("grounding is nondeterministic")
	}
	if grounding.Incomplete || len(grounding.Providers) != 2 {
		t.Fatalf("unexpected provider result: %+v", grounding)
	}
	for _, provider := range grounding.Providers {
		if !strings.HasPrefix(provider.InputID, "sha256:") || len(provider.Coverage.Limits) == 0 {
			t.Fatalf("provider lacks identity or coverage: %+v", provider)
		}
	}

	declared := findGroundedRelationship(grounding, "artifact:a:a", "artifact:a:b")
	if declared == nil || declared.Resolution != "declared" || declared.DeclarationIndex == nil {
		t.Fatalf("declared relationship lost provenance: %+v", declared)
	}
	ownership := findGroundedRelationship(grounding, "artifact:a:a", "owner-label")
	if ownership == nil || ownership.Resolution != "declared_ownership" ||
		ownership.Provider != "catalog" || ownership.TargetLabel != "maintainers" {
		t.Fatalf("ownership claim was not separated: %+v", ownership)
	}
	document := findGroundedRelationship(grounding, "file:README.md", "file:docs/guide.md#usage")
	if document == nil || document.Resolution != "syntactic" || !document.Sites[0].Verified {
		t.Fatalf("document link was not grounded: %+v", document)
	}
	dependency := findGroundedRelationship(
		grounding,
		"unit:go:app/app",
		"unit:go:lib/lib",
	)
	if dependency == nil || dependency.Resolution != "provider_resolved" || dependency.ImportAlias != "chosen" {
		t.Fatalf("Go import alias or resolution missing: %+v", dependency)
	}
	if findGroundedRelationship(grounding, "unit:go:app/app", "module:fmt") != nil {
		t.Fatal("external import exceeded provider coverage")
	}

	codes := make([]string, 0, len(grounding.Diagnostics))
	for _, diagnostic := range grounding.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	if !reflect.DeepEqual(codes, []string{"document_target_unavailable_in_index", "provider_target_unresolved"}) {
		t.Fatalf("unresolved targets were not explicit: %+v", grounding.Diagnostics)
	}
}

func TestGroundRelationshipsBoundsAndRejectsUnsupportedConfiguration(t *testing.T) {
	repo := groundingFixture(t)
	empty, err := GroundRelationships(repo, nil, GroundOptions{})
	if err != nil || len(empty.Relationships) != 0 || len(empty.Providers) != 0 || empty.Incomplete {
		t.Fatalf("source-only grounding changed behavior: grounding=%+v err=%v", empty, err)
	}
	grounding, err := GroundRelationships(repo, nil, GroundOptions{
		DocumentLinks:    true,
		GoImports:        &GoImportOptions{GoMod: []byte(groundingGoMod)},
		MaxRelationships: 1,
		MaxDiagnostics:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !grounding.Incomplete || len(grounding.Relationships) != 1 || len(grounding.Diagnostics) != 1 {
		t.Fatalf("grounding limits were not exact: %+v", grounding)
	}
	if grounding.Omissions.Relationships == 0 || grounding.Omissions.Diagnostics == 0 {
		t.Fatalf("grounding omissions were not counted: %+v", grounding.Omissions)
	}

	bad := []GroundOptions{
		{GoImports: &GoImportOptions{}},
		{GoImports: &GoImportOptions{GoMod: []byte("module example.test/other\n")}},
		{MaxRelationships: -1},
		{MaxDiagnostics: maximumGroundingLimit + 1},
	}
	for _, options := range bad {
		grounding, err := GroundRelationships(repo, nil, options)
		if err == nil || grounding != nil {
			t.Fatalf("unsupported configuration accepted: options=%+v grounding=%+v", options, grounding)
		}
	}
	if grounding, err := GroundRelationships(nil, nil, GroundOptions{}); err == nil || grounding != nil {
		t.Fatal("nil repository accepted")
	}

	invalidGoMod := "module ../escape\n"
	invalidModuleRepo := compileGroundingFixture(t, map[string]string{
		"go.mod":  invalidGoMod,
		"main.go": "package main\n",
	})
	grounding, err = GroundRelationships(invalidModuleRepo, nil, GroundOptions{
		GoImports: &GoImportOptions{GoMod: []byte(invalidGoMod)},
	})
	if err == nil || grounding != nil || !strings.Contains(err.Error(), "supported module directive") {
		t.Fatalf("unsupported module directive accepted: grounding=%+v err=%v", grounding, err)
	}

	incompleteRepo := compileGroundingFixture(t, map[string]string{
		"go.mod":    groundingGoMod,
		"broken.go": "package broken\nfunc Missing(\n",
	})
	grounding, err = GroundRelationships(incompleteRepo, nil, GroundOptions{
		GoImports: &GoImportOptions{GoMod: []byte(groundingGoMod)},
	})
	if err == nil || grounding != nil || !strings.Contains(err.Error(), "parse-incomplete") {
		t.Fatalf("parse-incomplete provider input accepted: grounding=%+v err=%v", grounding, err)
	}
}

func TestGroundRelationshipsDiagnosesDeclaredEndpoints(t *testing.T) {
	repo := groundingFixture(t)
	catalog := minimal()
	catalog.Artifacts = append(catalog.Artifacts, catalog.Artifacts[0])
	catalog.Relationships = append(catalog.Relationships, relationship("a:a", "a:missing", "references"))

	grounding, err := GroundRelationships(repo, catalog, GroundOptions{})
	if err != nil {
		t.Fatal(err)
	}
	codes := make([]string, 0, len(grounding.Diagnostics))
	for _, diagnostic := range grounding.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	if !reflect.DeepEqual(codes, []string{"declared_target_ambiguous", "declared_target_unresolved"}) ||
		len(grounding.Relationships) != 0 {
		t.Fatalf("declared endpoint failure was not explicit: %+v", grounding)
	}
}

func groundingFixture(t *testing.T) *ir.Repository {
	t.Helper()
	return compileGroundingFixture(t, map[string]string{
		"go.mod":        groundingGoMod,
		"README.md":     "# Repository\n\n[guide](docs/guide.md#usage) [missing](missing.md) [web](https://example.test)\n",
		"docs/guide.md": "# Guide\n\n## Usage\n",
		"app/main.go": "package app\nimport (\n chosen \"example.test/repo/lib\"\n \"example.test/repo/missing\"\n \"fmt\"\n)\n" +
			"func Run() { fmt.Println(chosen.Value) }\n",
		"lib/lib.go":          "package lib\nconst Value = 1\n",
		"lib/example_test.go": "package lib_test\n",
	})
}

const groundingGoMod = "module example.test/repo\n\ngo 1.23\n"

func compileGroundingFixture(t *testing.T, files map[string]string) *ir.Repository {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func findGroundedRelationship(grounding *Grounding, from, to string) *GroundedRelationship {
	for i := range grounding.Relationships {
		relationship := &grounding.Relationships[i]
		if relationship.From == from && relationship.To == to {
			return relationship
		}
	}
	return nil
}
