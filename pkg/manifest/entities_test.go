package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileEntitiesPreservesExactDeclarations(t *testing.T) {
	root := t.TempDir()
	writeData(t, root, "claims.json", validArtifactAuthoring)
	write(t, root, "go.mod")
	writeData(t, root, "model.md", validFrontmatter)
	writeData(t, root, "agent-context.yaml", validManifest)

	model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.Version != "repoctx.entities/v1alpha2" || model.Namespace != "demo" || len(model.Entities) != 3 {
		t.Fatalf("unexpected entity model: %+v", model)
	}
	if model.Entities[0].ID != "demo:contract.claim" || model.Entities[1].ID != "demo:core" || model.Entities[2].ID != "demo:maintainers" {
		t.Fatalf("entities are not canonical: %+v", model.Entities)
	}
	for _, entity := range model.Entities {
		if len(entity.Declaration.Sources) != 1 {
			t.Fatalf("unexpected declaration sources: %+v", entity.Declaration.Sources)
		}
		source := entity.Declaration.Sources[0]
		data, err := os.ReadFile(filepath.Join(root, source.Path))
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		if source.SHA256 != hex.EncodeToString(hash[:]) || source.StartByte < 0 || source.EndByte > int64(len(data)) {
			t.Fatalf("invalid source identity: %+v", source)
		}
		excerpt := data[source.StartByte:source.EndByte]
		localID := strings.TrimPrefix(entity.ID, "demo:")
		expected := "id: " + localID
		if entity.ID == "demo:contract.claim" {
			expected = "# Model"
		}
		if !strings.Contains(string(excerpt), expected) || entity.Declaration.Status != "declared" {
			t.Fatalf("declaration is not exact for %s: %q", entity.ID, excerpt)
		}
	}
	if len(model.Relationships) != 1 || model.Relationships[0].From != "demo:contract.claim" ||
		model.Relationships[0].To != "demo:core" || model.Relationships[0].Kind != "references" ||
		len(model.Relationships[0].Declaration.Sources) != 1 {
		t.Fatalf("artifact relationships were not compiled canonically: %+v", model.Relationships)
	}
}

func TestCompileEntitiesRejectsDuplicateIdentityAcrossInputs(t *testing.T) {
	duplicateArtifact := func() string {
		t.Helper()
		var document map[string]any
		if err := json.Unmarshal([]byte(validArtifactAuthoring), &document); err != nil {
			t.Fatal(err)
		}
		declarations := document["artifacts"].([]any)
		document["artifacts"] = append(declarations, declarations[0])
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	tests := map[string]string{
		"manifest component":   strings.Replace(validArtifactAuthoring, "demo:contract.claim", "demo:core", 2),
		"document frontmatter": strings.Replace(validArtifactAuthoring, "demo:contract.claim", "demo:maintainers", 2),
		"artifact catalog":     duplicateArtifact(),
	}
	for name, authoring := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeData(t, root, "claims.json", authoring)
			write(t, root, "go.mod")
			writeData(t, root, "model.md", validFrontmatter)
			writeData(t, root, "agent-context.yaml", validManifest)
			model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil)
			if err == nil || model != nil || !strings.Contains(err.Error(), "duplicate entity identity") {
				t.Fatalf("duplicate selected a declaration: model=%+v err=%v", model, err)
			}
		})
	}
}

func TestCompileEntitiesRejectsUnresolvedClaimsAndScope(t *testing.T) {
	root := t.TempDir()
	writeData(t, root, "claims.json", validArtifactAuthoring)
	write(t, root, "go.mod")
	writeData(t, root, "model.md", strings.Replace(validFrontmatter, "owners: []", "owners: [missing]", 1))
	writeData(t, root, "agent-context.yaml", validManifest)

	if model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil); err == nil || model != nil || !strings.Contains(err.Error(), "unknown owner entity") {
		t.Fatalf("unresolved owner accepted: model=%+v err=%v", model, err)
	}
	if model, err := CompileEntities(root, "agent-context.yaml", Limits{}, func(path string) bool {
		return path != "model.md"
	}); err == nil || model != nil || !strings.Contains(err.Error(), "outside caller scope") {
		t.Fatalf("denied document accepted: model=%+v err=%v", model, err)
	}
}

func TestCompileEntitiesUsesRelocationStableOutput(t *testing.T) {
	compile := func(root string) []byte {
		t.Helper()
		writeData(t, root, "claims.json", validArtifactAuthoring)
		write(t, root, "go.mod")
		writeData(t, root, "model.md", validFrontmatter)
		writeData(t, root, "agent-context.yaml", validManifest)
		model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	if first, second := compile(t.TempDir()), compile(t.TempDir()); string(first) != string(second) {
		t.Fatalf("relocation changed semantic output: %s != %s", first, second)
	}
}
