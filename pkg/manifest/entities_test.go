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
	write(t, root, "claims.json")
	write(t, root, "go.mod")
	writeData(t, root, "model.md", validFrontmatter)
	writeData(t, root, "agent-context.yaml", validManifest)

	model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model.Version != "repoctx.entities/v1alpha1" || model.Namespace != "demo" || len(model.Entities) != 2 {
		t.Fatalf("unexpected entity model: %+v", model)
	}
	if model.Entities[0].ID != "demo:core" || model.Entities[1].ID != "demo:maintainers" {
		t.Fatalf("entities are not canonical: %+v", model.Entities)
	}
	for _, entity := range model.Entities {
		data, err := os.ReadFile(filepath.Join(root, entity.Declaration.Source.Path))
		if err != nil {
			t.Fatal(err)
		}
		source := entity.Declaration.Source
		hash := sha256.Sum256(data)
		if source.SHA256 != hex.EncodeToString(hash[:]) || source.StartByte < 0 || source.EndByte > int64(len(data)) {
			t.Fatalf("invalid source identity: %+v", source)
		}
		excerpt := data[source.StartByte:source.EndByte]
		localID := strings.TrimPrefix(entity.ID, "demo:")
		if !strings.Contains(string(excerpt), "id: "+localID) || entity.Declaration.Status != "declared" {
			t.Fatalf("declaration is not exact for %s: %q", entity.ID, excerpt)
		}
	}
}

func TestCompileEntitiesRejectsUnresolvedClaimsAndScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "claims.json")
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
		write(t, root, "claims.json")
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
