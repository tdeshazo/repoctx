package obligation

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestBuildFromRepositoryConsumesCanonicalDeclarations(t *testing.T) {
	root, repository := canonicalFixture(t)
	declaration := repository.Entities.Entities[0].Declaration
	repository.Entities.Relationships = []ir.EntityRelationship{
		{From: "demo:core", To: "demo:requirement", Kind: "governed_by", Declaration: declaration},
		{From: "demo:obligation", To: "demo:requirement", Kind: "verifies", Declaration: declaration},
	}
	context := canonicalContext(t, root, repository)
	before, err := repository.SnapshotID()
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildFromRepository(context, repository, canonicalAuthority(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := BuildFromRepository(context, repository, canonicalAuthority(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Payload, repeated.Payload) {
		t.Fatal("canonical handoff is nondeterministic")
	}
	if len(result.Bundle.Artifacts) != 3 || len(result.Bundle.Relationships) != 2 {
		t.Fatalf("canonical declarations were not selected: %+v", result.Bundle)
	}
	if result.Bundle.SourceSnapshot != before || result.Bundle.SourceTaskID != context.Bundle.TaskID {
		t.Fatal("canonical handoff lost context identity")
	}
	for _, artifact := range result.Bundle.Artifacts {
		if artifact.RunnerCheckID != nil || artifact.Sources[0].Verification != "declared_unverified" {
			t.Fatalf("canonical metadata gained runner or source authority: %+v", artifact)
		}
		if len(artifact.Scopes) != 1 || artifact.Scopes[0].Path != "main.go" {
			t.Fatalf("caller scope was widened: %+v", artifact.Scopes)
		}
	}
	for _, relationship := range result.Bundle.Relationships {
		if relationship.Resolution != "declared" {
			t.Fatalf("relationship gained authority: %+v", relationship)
		}
	}
	after, err := repository.SnapshotID()
	if err != nil || after != before {
		t.Fatal("obligation planning mutated the canonical repository")
	}
	// Planning uses the retained index/context only, even when authored files drift.
	if err := os.WriteFile(filepath.Join(root, "model.md"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	repeated, err = BuildFromRepository(context, repository, canonicalAuthority(), Options{})
	if err != nil || !bytes.Equal(result.Payload, repeated.Payload) {
		t.Fatalf("obligation planning unexpectedly reread declaration sources: %v", err)
	}
}

func TestBuildFromRepositoryRequiresCallerAuthority(t *testing.T) {
	root, repository := canonicalFixture(t)
	context := canonicalContext(t, root, repository)
	for _, authority := range []artifacts.Authority{
		{},
		{AcceptedIDs: canonicalAuthority().AcceptedIDs, Scopes: []artifacts.Applicability{{Kind: "file", Path: "other.go"}}},
	} {
		result, err := BuildFromRepository(context, repository, authority, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Bundle.Artifacts) != 0 || len(result.Bundle.Relationships) != 0 {
			t.Fatal("canonical claims granted themselves authority")
		}
	}
	authority := canonicalAuthority()
	authority.AcceptedIDs = []string{"demo:obligation"}
	result, err := BuildFromRepository(context, repository, authority, Options{})
	if err != nil || len(result.Bundle.Artifacts) != 1 || result.Bundle.Artifacts[0].ID != "demo:obligation" {
		t.Fatalf("accepted ID boundary was not retained: %v, %+v", err, result)
	}
	authority.AcceptedIDs = []string{"demo:owner"}
	if _, err := BuildFromRepository(context, repository, authority, Options{}); err == nil {
		t.Fatal("owner entity was accepted as a planning artifact")
	}
}

func TestBuildFromRepositoryRejectsDifferentSnapshot(t *testing.T) {
	root, repository := canonicalFixture(t)
	context := canonicalContext(t, root, repository)
	repository.Entities.Entities[0].Lifecycle = "retired"
	if _, err := BuildFromRepository(context, repository, canonicalAuthority(), Options{}); err == nil {
		t.Fatal("canonical entities from another snapshot were accepted")
	}
	if _, err := BuildFromRepository(context, nil, canonicalAuthority(), Options{}); err == nil {
		t.Fatal("nil repository accepted")
	}
	if _, err := BuildFromRepository(nil, repository, canonicalAuthority(), Options{}); err == nil {
		t.Fatal("nil context accepted")
	}
	repository.Entities = nil
	if _, err := BuildFromRepository(context, repository, canonicalAuthority(), Options{}); err == nil {
		t.Fatal("missing canonical entities accepted")
	}
}

func TestBuildFromRepositoryPreservesSupersessionConflicts(t *testing.T) {
	root, repository := canonicalFixture(t)
	model := repository.Entities
	prior := model.Entities[len(model.Entities)-1]
	prior.ID = "demo:prior"
	model.Entities[len(model.Entities)-1].Supersedes = []string{prior.ID}
	model.Entities = append(model.Entities, prior)
	slices.SortFunc(model.Entities, func(a, b ir.Entity) int { return strings.Compare(a.ID, b.ID) })
	context := canonicalContext(t, root, repository)
	authority := canonicalAuthority()
	authority.AcceptedIDs = []string{prior.ID, "demo:requirement"}
	result, err := BuildFromRepository(context, repository, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 0 || !slices.Contains(diagnosticCodes(result), "accepted_supersession_conflict") {
		t.Fatalf("canonical supersedes claim selected an authoritative winner: %+v", result.Bundle)
	}
}

func TestPlanningCatalogPreservesFreshnessForInputConflicts(t *testing.T) {
	root, repository := canonicalFixture(t)
	model := repository.Entities
	requirement := &model.Entities[len(model.Entities)-1]
	requirement.Freshness.Inputs = []string{"main.go", "config/build.json"}
	other := *requirement
	other.ID = "demo:other-requirement"
	model.Entities = append(model.Entities, other)
	slices.SortFunc(model.Entities, func(a, b ir.Entity) int { return strings.Compare(a.ID, b.ID) })
	context := canonicalContext(t, root, repository)
	authority := canonicalAuthority()
	authority.AcceptedIDs = []string{other.ID, "demo:requirement"}

	// Shared freshness paths alone are compatible: canonical metadata contains
	// neither contradictory roles nor required flags nor expected digests.
	result, err := BuildFromRepository(context, repository, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 2 || len(result.Bundle.Diagnostics) != 0 {
		t.Fatalf("freshness overlap manufactured a conflict: %+v", result.Bundle)
	}
	catalog, err := planningCatalog(model, authority.AcceptedIDs)
	if err != nil {
		t.Fatal(err)
	}
	expected := []artifacts.Input{
		{Path: "main.go", Role: "source", Required: true},
		{Path: "config/build.json", Role: "source", Required: true},
	}
	for i := range catalog.Artifacts {
		artifact := &catalog.Artifacts[i]
		if !slices.Contains(authority.AcceptedIDs, artifact.ID) {
			continue
		}
		if !slices.Equal(artifact.DeclaredInputs, expected) {
			t.Fatalf("freshness order or declaration meaning lost: %+v", artifact.DeclaredInputs)
		}
		if artifact.ID == other.ID {
			// Supply the explicit contradictory claim that canonical path-only
			// freshness cannot express, then exercise the existing resolver.
			artifact.DeclaredInputs[0].Role = "configuration"
		}
	}
	result, err = Build(context, catalog, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 0 || len(result.Bundle.Diagnostics) != 1 {
		t.Fatalf("projected freshness bypassed conflicting-input resolution: %+v", result.Bundle)
	}
	diagnostic := result.Bundle.Diagnostics[0]
	if diagnostic.Code != "conflicting_accepted_requirements" || diagnostic.Path != "main.go" ||
		!slices.Equal(diagnostic.ArtifactIDs, authority.AcceptedIDs) {
		t.Fatalf("projected freshness lost conflict provenance: %+v", diagnostic)
	}
}

func TestBuildFromRepositoryBoundsHandoff(t *testing.T) {
	root, repository := canonicalFixture(t)
	context := canonicalContext(t, root, repository)
	result, err := BuildFromRepository(context, repository, canonicalAuthority(), Options{MaxArtifacts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 1 || result.Bundle.Omissions.Artifacts != 2 {
		t.Fatalf("artifact count limit lost: %+v", result.Bundle)
	}
	const maxBytes = 1000
	result, err = BuildFromRepository(context, repository, canonicalAuthority(), Options{MaxBytes: maxBytes})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Payload) > maxBytes || result.Bundle.Omissions.Budget == 0 {
		t.Fatalf("canonical handoff exceeded or ignored its budget: %+v", result.Bundle)
	}
}

func canonicalAuthority() artifacts.Authority {
	return artifacts.Authority{
		AcceptedIDs: []string{"demo:core", "demo:obligation", "demo:requirement"},
		Scopes:      []artifacts.Applicability{{Kind: "file", Path: "main.go"}},
	}
}

func canonicalContext(t *testing.T, root string, repository *ir.Repository) *agentctx.Result {
	t.Helper()
	result, err := agentctx.Build(repository, agentctx.Options{Root: root, Query: "Run", MaxBytes: 12000})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func canonicalFixture(t *testing.T) (string, *ir.Repository) {
	t.Helper()
	root := t.TempDir()
	document := "---\nversion: repoctx.frontmatter/v1alpha1\nentities:\n"
	for _, entity := range []struct{ id, kind string }{
		{id: "document", kind: "document"},
		{id: "obligation", kind: "verification_obligation"},
		{id: "owner", kind: "owner"},
		{id: "requirement", kind: "requirement"},
	} {
		document += "  - id: " + entity.id + "\n    kind: " + entity.kind + `
    owners: []
    scopes: [{kind: file, path: main.go}]
    lifecycle: active
    supersedes: []
    sensitivity: internal
    freshness: {inputs: [main.go]}
`
	}
	document += "---\n# Declared model\n"
	for path, contents := range map[string]string{
		"main.go":  "package demo\n\nfunc Run() string { return \"ok\" }\n",
		"model.md": document,
		"agent-context.yaml": `version: repoctx.manifest/v1alpha3
namespace: demo
source_roots: [{id: source, path: .}]
document_sources: [{id: semantics, path: model.md, format: repoctx.frontmatter/v1alpha1}]
artifact_sources: []
components:
  - id: core
    source_roots: [source]
    artifact_sources: []
    provider_inputs: []
    owners: [owner]
    scopes: [{kind: subtree, path: .}]
    lifecycle: active
    supersedes: []
    sensitivity: internal
    freshness: {inputs: [main.go]}
provider_inputs: []
derived_views: [{id: index, kind: repository_ir, components: [core]}]
capabilities: [syntax_graph, semantic_entities]
`,
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	repository, err := compiler.Compile(compiler.Options{Root: root, Manifest: "agent-context.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	return root, repository
}
