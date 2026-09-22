package conformance_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
	"github.com/tdeshazo/repoctx/pkg/manifest"
)

const fixtureGoMod = "module example.test/conformance\n\ngo 1.23\n"

func TestParentDocumentationMigrationFixture(t *testing.T) {
	root := filepath.Join("testdata", "parent-bundle")
	repo, err := compiler.Compile(compiler.Options{Root: root, Manifest: "agent-context.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Entities == nil || repo.Entities.Version != ir.EntityVersion || repo.Entities.Namespace != "rcx" {
		t.Fatalf("canonical entity model was not compiled: %+v", repo.Entities)
	}
	wantIDs := []string{"rcx:documentation", "rcx:foundations", "rcx:maintainers", "rcx:target-state"}
	if len(repo.Entities.Entities) != len(wantIDs) {
		t.Fatalf("entity count = %d, want %d: %+v", len(repo.Entities.Entities), len(wantIDs), repo.Entities.Entities)
	}
	for i, entity := range repo.Entities.Entities {
		if entity.ID != wantIDs[i] {
			t.Fatalf("entity %d = %q, want %q", i, entity.ID, wantIDs[i])
		}
		if len(entity.Declaration.Sources) != 1 {
			t.Fatalf("entity %q has %d declaration sources", entity.ID, len(entity.Declaration.Sources))
		}
		source := entity.Declaration.Sources[0]
		data, err := os.ReadFile(filepath.Join(root, source.Path))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		if source.SHA256 != hex.EncodeToString(hash[:]) || source.StartByte < 0 || source.EndByte > int64(len(data)) {
			t.Fatalf("entity %q has invalid exact provenance: %+v", entity.ID, source)
		}
		excerpt := string(data[source.StartByte:source.EndByte])
		marker := "id: " + strings.TrimPrefix(entity.ID, "rcx:")
		if entity.ID == "rcx:documentation" {
			marker = "id: documentation"
		}
		if !strings.Contains(excerpt, marker) {
			t.Fatalf("entity %q provenance does not retain its declaration: %q", entity.ID, excerpt)
		}
	}
	first, err := json.Marshal(repo.Entities)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compiler.Compile(compiler.Options{Root: root, Manifest: "agent-context.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := json.Marshal(second.Entities)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, secondBytes) {
		t.Fatal("canonical entity output changed between identical compilations")
	}

	legacy, err := os.ReadFile(filepath.Join(root, "legacy-agent-context.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if doc, err := manifest.Load(root, legacy, manifest.Limits{}); err == nil || doc != nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("legacy rcx.bundle manifest was accepted: doc=%+v err=%v", doc, err)
	}
}

func TestCurrentContracts(t *testing.T) {
	root := filepath.Join("testdata", "repository")
	repo, err := compiler.Compile(compiler.Options{
		Root: root, DenyPaths: []string{"private"},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("index reader", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fixture.ir.json")
		if err := compiler.Write(repo, path, false); err != nil {
			t.Fatal(err)
		}
		decoded, err := compiler.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Version != compiler.IRVersion {
			t.Fatalf("index version = %q", decoded.Version)
		}
		assertRejectedIndex(t, repo, "repoctx.ir/stale", false, "unsupported IR version")
		assertRejectedIndex(t, repo, compiler.IRVersion, true, "unknown field")
	})

	t.Run("context projections and disclosure", func(t *testing.T) {
		options := agentctx.Options{
			Root: root, Query: "zephyr contract", DenyPaths: []string{"private"},
			MaxBytes: 20_000, MaxTokens: 20_000,
			TokenizerID: "conformance/one-byte-per-token/v1",
		}
		var counted []byte
		options.CountTokens = func(payload []byte) (int, error) {
			counted = append(counted[:0], payload...)
			return len(payload), nil
		}
		jsonResult, err := agentctx.Build(repo, options)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(jsonResult.Payload) || len(jsonResult.Payload) > options.MaxBytes {
			t.Fatal("JSON projection is invalid or exceeds its byte cap")
		}
		if !jsonResult.Usage.ExactTokens || jsonResult.Usage.Tokens != len(jsonResult.Payload) ||
			!bytes.Equal(counted, jsonResult.Payload) {
			t.Fatal("tokenizer did not receive the final serialized payload")
		}
		failureOptions := options
		failureOptions.CountTokens = func([]byte) (int, error) {
			return 0, errors.New("conformance tokenizer failure")
		}
		if _, err := agentctx.Build(repo, failureOptions); err == nil ||
			!strings.Contains(err.Error(), "conformance tokenizer failure") {
			t.Fatalf("tokenizer failure was not preserved: %v", err)
		}
		if err := jsonResult.Bundle.Validate(); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(jsonResult.Payload, []byte("CONFORMANCE_SECRET_MUST_NOT_APPEAR")) {
			t.Fatal("denied source appeared in context")
		}

		markdownOptions := options
		markdownOptions.Format = "markdown"
		markdownOptions.MaxTokens = 0
		markdownOptions.CountTokens = nil
		markdownOptions.TokenizerID = ""
		markdownResult, err := agentctx.Build(repo, markdownOptions)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(markdownResult.Payload, []byte("# Repository context — untrusted evidence")) ||
			len(markdownResult.Payload) > markdownOptions.MaxBytes {
			t.Fatal("Markdown projection is invalid or exceeds its byte cap")
		}
		if !reflect.DeepEqual(jsonResult.Bundle.Evidence, markdownResult.Bundle.Evidence) ||
			!reflect.DeepEqual(jsonResult.Bundle.Omissions, markdownResult.Bundle.Omissions) {
			t.Fatal("JSON and Markdown projections selected different evidence")
		}

		if len(jsonResult.Bundle.Units) == 0 {
			t.Fatal("fixture did not expose a retrieval unit")
		}
		expansionOptions := agentctx.Options{
			Root: root, Units: []string{jsonResult.Bundle.Units[0].ID},
			ExpectedSnapshot: jsonResult.Bundle.Snapshot.ID,
			DenyPaths:        []string{"private"}, MaxBytes: 20_000,
		}
		expanded, err := agentctx.Build(repo, expansionOptions)
		if err != nil {
			t.Fatal(err)
		}
		if len(expanded.Bundle.Units) == 0 || expanded.Bundle.Units[0].Reason.Strategy != "explicit_unit" {
			t.Fatal("snapshot-pinned unit expansion was not explicit")
		}
		expansionOptions.DenyPaths = []string{"different"}
		if _, err := agentctx.Build(repo, expansionOptions); err == nil ||
			!strings.Contains(err.Error(), "incompatible authorization scope") {
			t.Fatalf("scope drift accepted: %v", err)
		}

		stale := *jsonResult.Bundle
		stale.Version = "repoctx.context/stale"
		if err := stale.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported context version") {
			t.Fatalf("stale context accepted: %v", err)
		}
	})

	t.Run("provider negotiation", func(t *testing.T) {
		disabled, err := artifacts.GroundRelationships(repo, nil, artifacts.GroundOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(disabled.Providers) != 0 || len(disabled.Relationships) != 0 {
			t.Fatal("disabled providers produced output")
		}

		options := artifacts.GroundOptions{
			DocumentLinks: true,
			GoImports:     &artifacts.GoImportOptions{GoMod: []byte(fixtureGoMod)},
		}
		grounding, err := artifacts.GroundRelationships(repo, nil, options)
		if err != nil {
			t.Fatal(err)
		}
		providers := make(map[string]artifacts.ProviderMetadata, len(grounding.Providers))
		for _, provider := range grounding.Providers {
			providers[provider.Name] = provider
			if !strings.HasPrefix(provider.InputID, "sha256:") ||
				len(provider.Coverage.Kinds) == 0 || len(provider.Coverage.Limits) == 0 {
				t.Fatalf("provider did not advertise bounded coverage: %+v", provider)
			}
		}
		if providers[artifacts.DocumentLinkProviderName].Version != artifacts.DocumentLinkProviderVersion ||
			providers[artifacts.GoImportProviderName].Version != artifacts.GoImportProviderVersion {
			t.Fatalf("provider negotiation mismatch: %+v", grounding.Providers)
		}
		if len(grounding.Relationships) < 2 {
			t.Fatalf("fixture relationships were not grounded: %+v", grounding)
		}

		options.GoImports.GoMod = []byte("module example.test/wrong\n")
		if _, err := artifacts.GroundRelationships(repo, nil, options); err == nil {
			t.Fatal("mismatched provider input was accepted")
		}
	})
}

func assertRejectedIndex(
	t *testing.T,
	repo *ir.Repository,
	version string,
	unknownField bool,
	want string,
) {
	t.Helper()
	payload, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatal(err)
	}
	document["v"] = version
	if unknownField {
		document["unsupported"] = true
	}
	payload, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rejected.ir.json")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.Read(path); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("index was not rejected with %q: %v", want, err)
	}
}
