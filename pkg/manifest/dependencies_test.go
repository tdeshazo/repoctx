package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const dependencyComponents = `  - id: storage
    source_roots: [source]
    artifact_sources: []
    provider_inputs: []
    owners: []
    scopes: []
    lifecycle: active
    supersedes: []
    sensitivity: internal
    freshness: {inputs: []}
  - id: database
    source_roots: [source]
    artifact_sources: []
    provider_inputs: []
    owners: []
    scopes: []
    lifecycle: active
    supersedes: []
    sensitivity: internal
    freshness: {inputs: []}
`

func manifestWithDependencies(field string) string {
	data := strings.Replace(validManifest, "    owners: [maintainers]", field+"    owners: [maintainers]", 1)
	return strings.Replace(data, "provider_inputs:\n  - id: module", dependencyComponents+"provider_inputs:\n  - id: module", 1)
}

func TestLoadComponentDependencies(t *testing.T) {
	root := t.TempDir()
	write(t, root, "claims.json")
	write(t, root, "go.mod")
	writeData(t, root, "model.md", validFrontmatter)

	for _, field := range []string{"", "    depends_on: []\n", "    depends_on: [storage, database]\n"} {
		doc, err := Load(root, []byte(manifestWithDependencies(field)), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if strings.Contains(field, "storage") {
			want = 2
		}
		if got := len(doc.Components[0].DependsOn); got != want {
			t.Fatalf("dependency count = %d, want %d", got, want)
		}
	}

	tests := []struct {
		name   string
		field  string
		reason string
	}{
		{name: "unknown", field: "[missing]", reason: "unknown component"},
		{name: "wrong kind", field: "[source]", reason: "unknown component"},
		{name: "external owner", field: "[maintainers]", reason: "unknown component"},
		{name: "self", field: "[core]", reason: "self dependency"},
		{name: "duplicate", field: "[storage, storage]", reason: "duplicate component"},
		{name: "null", field: "null", reason: "invalid field type"},
		{name: "scalar", field: "storage", reason: "invalid field type"},
		{name: "coercion", field: "[42]", reason: "expected string"},
		{name: "object", field: "[{id: storage}]", reason: "invalid field type"},
		{name: "anchor", field: "[&dependency storage]", reason: "aliases and anchors"},
		{name: "tag", field: "[!!str storage]", reason: "explicit YAML tags"},
		{
			name: "count", field: "[" + strings.Repeat("storage, ", 256) + "database]",
			reason: "count limit exceeded",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := manifestWithDependencies("    depends_on: " + test.field + "\n")
			doc, err := Load(root, []byte(data), Limits{})
			if err == nil || doc != nil {
				t.Fatalf("invalid dependency accepted: doc=%+v err=%v", doc, err)
			}
			if !strings.Contains(err.Error(), test.reason) || len(err.Error()) > 256 {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
		})
	}
}

func TestCompileComponentDependenciesPreservesExactDeclarations(t *testing.T) {
	for name, field := range map[string]string{
		"flow":             "    depends_on: [storage, database]\n",
		"block":            "    depends_on:\n      - storage\n      - database\n",
		"folded scalars":   "    depends_on:\n      - >-\n        storage\n      - >-\n        database\n",
		"continued quotes": "    depends_on:\n      - \"sto\\\n        rage\"\n      - \"data\\\n        base\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			lastField := "    freshness: {inputs: [go.mod]}\n"
			data := strings.Replace(manifestWithDependencies(""), lastField, lastField+field, 1)
			compile := func() []byte {
				t.Helper()
				root := t.TempDir()
				writeData(t, root, "claims.json", validArtifactAuthoring)
				write(t, root, "go.mod")
				writeData(t, root, "model.md", validFrontmatter)
				writeData(t, root, "agent-context.yaml", data)

				model, err := CompileEntities(root, "agent-context.yaml", Limits{}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(model.Relationships) != 3 {
					t.Fatalf("unexpected relationships: %+v", model.Relationships)
				}
				hash := sha256.Sum256([]byte(data))
				start, end := strings.Index(data, "  - id: core\n"), strings.Index(data, "  - id: storage\n")
				for i, target := range []string{"demo:database", "demo:storage"} {
					relationship := model.Relationships[i+1]
					if relationship.From != "demo:core" || relationship.To != target || relationship.Kind != "depends_on" {
						t.Fatalf("dependency is not canonical: %+v", relationship)
					}
					if relationship.Declaration.Status != "declared" || len(relationship.Declaration.Sources) != 1 {
						t.Fatalf("unexpected declaration: %+v", relationship.Declaration)
					}
					source := relationship.Declaration.Sources[0]
					if source.Path != "agent-context.yaml" || source.SHA256 != hex.EncodeToString(hash[:]) ||
						source.StartByte != int64(start) || source.EndByte != int64(end) {
						t.Fatalf("dependency does not cite its exact component declaration: %+v", source)
					}
				}
				encoded, err := json.Marshal(model)
				if err != nil {
					t.Fatal(err)
				}
				return encoded
			}
			if first, second := compile(), compile(); string(first) != string(second) {
				t.Fatalf("relocation changed dependency output: %s != %s", first, second)
			}
		})
	}
}
