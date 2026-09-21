package obligation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compiler"
)

func TestBuildSelectsApplicableDeclarationsWithoutExecution(t *testing.T) {
	context, catalog := fixture(t)
	authority := artifacts.Authority{
		AcceptedIDs: []string{"demo:component", "demo:contract", "demo:obligation"},
		Scopes:      []artifacts.Applicability{{Kind: "subtree", Path: "."}},
	}
	result, err := Build(context, catalog, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := Build(context, catalog, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.Payload, repeated.Payload) || !json.Valid(result.Payload) {
		t.Fatal("obligation handoff is invalid or nondeterministic")
	}
	if len(result.Bundle.Artifacts) != 3 || len(result.Bundle.Relationships) != 2 {
		t.Fatalf("unexpected selection: %+v", result.Bundle)
	}
	if result.Bundle.SourceTaskID != context.Bundle.TaskID || result.Bundle.SourceSnapshot != context.Bundle.Snapshot.ID {
		t.Fatal("source identities were not bound")
	}
	for _, artifact := range result.Bundle.Artifacts {
		if len(artifact.Sources) != 1 || artifact.Sources[0].Verification != "context_exact" || artifact.Sources[0].EvidenceID == "" {
			t.Fatalf("exact source provenance was not retained: %+v", artifact)
		}
		if artifact.Kind == "verification_obligation" {
			if artifact.RunnerCheckID == nil || *artifact.RunnerCheckID != "go:test/pkg" {
				t.Fatalf("opaque check id missing: %+v", artifact)
			}
		}
	}
	for _, relationship := range result.Bundle.Relationships {
		if relationship.Resolution != "declared" {
			t.Fatalf("relationship was promoted beyond its evidence: %+v", relationship)
		}
	}
	payload := string(result.Payload)
	for _, forbidden := range []string{`"status"`, `"exit_status"`, `"command"`, `"passed"`} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("handoff manufactured execution field %s", forbidden)
		}
	}
	if !strings.Contains(payload, "check_results") || !strings.Contains(payload, "passing_status") {
		t.Fatal("execution boundary is not explicit")
	}
}

func TestBuildRequiresAuthorityAndEvidenceApplicability(t *testing.T) {
	context, catalog := fixture(t)
	result, err := Build(context, catalog, artifacts.Authority{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 0 || len(result.Bundle.Relationships) != 0 {
		t.Fatal("repository declarations granted themselves authority")
	}

	authority := artifacts.Authority{
		AcceptedIDs: []string{"demo:component", "demo:contract", "demo:obligation"},
		Scopes:      []artifacts.Applicability{{Kind: "subtree", Path: "other"}},
	}
	result, err = Build(context, catalog, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 0 || !reflect.DeepEqual(diagnosticCodes(result), []string{
		"invalid_effective_scope", "invalid_effective_scope", "invalid_effective_scope",
	}) {
		t.Fatalf("invalid scope became applicable: %+v", result.Bundle)
	}
}

func TestBuildKeepsUncoveredProvenanceUnverified(t *testing.T) {
	context, catalog := fixture(t)
	contract := &catalog.Artifacts[1]
	contract.Sources[0].Path = "docs/contract.md"
	contract.Sources[0].SHA256 = strings.Repeat("0", 64)
	authority := artifacts.Authority{
		AcceptedIDs: []string{contract.ID},
		Scopes:      []artifacts.Applicability{{Kind: "subtree", Path: "."}},
	}
	result, err := Build(context, catalog, authority, Options{})
	if err != nil {
		t.Fatal(err)
	}
	source := result.Bundle.Artifacts[0].Sources[0]
	if source.Verification != "declared_unverified" || source.EvidenceID != "" {
		t.Fatalf("uncovered declaration source was promoted: %+v", source)
	}
}

func TestBuildReportsConflictsAndBoundsOutput(t *testing.T) {
	context, catalog := fixture(t)
	duplicate := catalog.Artifacts[1]
	catalog.Artifacts = append(catalog.Artifacts, duplicate)
	authority := artifacts.Authority{
		AcceptedIDs: []string{"demo:component", "demo:contract", "demo:obligation"},
		Scopes:      []artifacts.Applicability{{Kind: "subtree", Path: "."}},
	}
	result, err := Build(context, catalog, authority, Options{MaxArtifacts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bundle.Artifacts) != 1 || result.Bundle.Omissions.Artifacts != 1 {
		t.Fatalf("artifact limit was not explicit: %+v", result.Bundle)
	}
	if !reflect.DeepEqual(diagnosticCodes(result), []string{"ambiguous_reference", "ambiguous_reference", "duplicate_id"}) {
		t.Fatalf("conflicts were not retained: %+v", result.Bundle.Diagnostics)
	}
	if len(result.Payload) > defaultMaxBytes {
		t.Fatal("payload exceeded default budget")
	}
}

func TestBuildRejectsInvalidInputsAndTinyBudget(t *testing.T) {
	context, catalog := fixture(t)
	cases := []struct {
		context *agentctx.Result
		catalog *artifacts.Catalog
		options Options
	}{
		{nil, catalog, Options{}},
		{context, nil, Options{}},
		{context, catalog, Options{MaxArtifacts: -1}},
		{context, catalog, Options{MaxBytes: 1}},
	}
	for _, test := range cases {
		if result, err := Build(test.context, test.catalog, artifacts.Authority{}, test.options); err == nil || result != nil {
			t.Fatalf("invalid input accepted: result=%+v err=%v", result, err)
		}
	}
}

func fixture(t *testing.T) (*agentctx.Result, *artifacts.Catalog) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package demo\n\nfunc Run() string { return \"ok\" }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	repo, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	context, err := agentctx.Build(repo, agentctx.Options{Root: root, Query: "Run", MaxBytes: 12000})
	if err != nil {
		t.Fatal(err)
	}
	var evidence agentctx.Evidence
	for _, candidate := range context.Bundle.Evidence {
		if candidate.Role == "source" {
			evidence = candidate
			break
		}
	}
	if evidence.ID == "" {
		t.Fatal("fixture has no source evidence")
	}
	span := artifacts.Span{
		Path: "main.go", SHA256: evidence.SHA256,
		StartByte: int64(evidence.StartByte), EndByte: int64(evidence.EndByte),
		StartLine: int64(evidence.Span.StartLine), EndLine: int64(evidence.Span.EndLine),
		StartByteColumn: int64(evidence.Span.StartByteColumn), EndByteColumn: int64(evidence.Span.EndByteColumn),
	}
	checkID := "go:test/pkg"
	declaration := func(id, kind string) artifacts.Artifact {
		return artifacts.Artifact{
			ID: id, Kind: kind,
			AppliesTo: []artifacts.Applicability{{Kind: "file", Path: "main.go"}},
			Lifecycle: "active", Sources: []artifacts.Span{span}, DeclaredInputs: []artifacts.Input{},
		}
	}
	component := declaration("demo:component", "component")
	contract := declaration("demo:contract", "contract")
	obligation := declaration("demo:obligation", "verification_obligation")
	obligation.RunnerCheckID = &checkID
	return context, &artifacts.Catalog{
		Version: artifacts.Version, Namespace: "demo",
		Artifacts: []artifacts.Artifact{component, contract, obligation},
		Relationships: []artifacts.Relationship{
			{From: component.ID, To: contract.ID, Kind: "governed_by", Resolution: "declared", Sources: []artifacts.Span{span}},
			{From: obligation.ID, To: contract.ID, Kind: "verifies", Resolution: "declared", Sources: []artifacts.Span{span}},
		},
	}
}

func diagnosticCodes(result *Result) []string {
	codes := make([]string, 0, len(result.Bundle.Diagnostics))
	for _, diagnostic := range result.Bundle.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}
