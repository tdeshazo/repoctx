package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `version: repoctx.manifest/v1alpha1
namespace: demo
source_roots:
  - id: source
    path: .
artifact_sources:
  - id: claims
    path: claims.json
    format: repoctx.artifact-authoring/v1alpha1
components:
  - id: core
    source_roots: [source]
    artifact_sources: [claims]
    provider_inputs: [module]
provider_inputs:
  - id: module
    provider: repoctx.go-imports/v1
    path: go.mod
derived_views:
  - id: index
    kind: repository_ir
    components: [core]
capabilities: [syntax_graph, artifact_declarations, go_imports]
`

func TestLoadValidManifestAndVerifyAvailability(t *testing.T) {
	root := t.TempDir()
	write(t, root, "claims.json")
	write(t, root, "go.mod")
	doc, err := Load(root, []byte(validManifest), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != Version || doc.Namespace != "demo" || len(doc.Components) != 1 || doc.Components[0].ID != "core" {
		t.Fatalf("unexpected manifest: %+v", doc)
	}
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if doc, err := Load(root, []byte(validManifest), Limits{}); err == nil || doc != nil ||
		!strings.Contains(err.Error(), "$.provider_inputs[0].path: provider input is unavailable") {
		t.Fatalf("unavailable input accepted: doc=%+v err=%v", doc, err)
	}
}

func TestLoadRejectsSymlinkedArtifactInput(t *testing.T) {
	root := t.TempDir()
	write(t, root, "target")
	write(t, root, "go.mod")
	if err := os.Symlink("target", filepath.Join(root, "claims.json")); err != nil {
		t.Fatal(err)
	}
	if doc, err := Load(root, []byte(validManifest), Limits{}); err == nil || doc != nil ||
		!strings.Contains(err.Error(), "$.artifact_sources[0].path: artifact source is unavailable") {
		t.Fatalf("symlinked input accepted: doc=%+v err=%v", doc, err)
	}
}

func TestLoadFileConfinesManifestToRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte(validManifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside.yaml", "link.yaml"} {
		doc, err := LoadFile(root, path, Limits{})
		if err == nil || doc != nil || len(err.Error()) > 256 {
			t.Fatalf("manifest path %q escaped root: doc=%+v err=%v", path, doc, err)
		}
	}
}

func TestRejectsUnsafeYAMLAndClosedShape(t *testing.T) {
	cases := map[string]string{
		"duplicate":                   strings.Replace(validManifest, "version: repoctx.manifest/v1alpha1", "version: repoctx.manifest/v1alpha1\nversion: repoctx.manifest/v1alpha1", 1),
		"alias":                       strings.Replace(validManifest, "- id: source", "- &root\n    id: source", 1),
		"explicit tag":                strings.Replace(validManifest, "version: repoctx.manifest/v1alpha1", "version: !!str repoctx.manifest/v1alpha1", 1),
		"unknown":                     strings.Replace(validManifest, "capabilities:", "generated_sha256: nope\ncapabilities:", 1),
		"unsafe path":                 strings.Replace(validManifest, "path: claims.json", "path: ../claims.json", 1),
		"unknown reference":           strings.Replace(validManifest, "source_roots: [source]", "source_roots: [missing]", 1),
		"duplicate identity":          strings.Replace(validManifest, "id: claims", "id: source", 1),
		"unsupported version":         strings.Replace(validManifest, Version, "repoctx.manifest/v0", 1),
		"scalar coercion":             strings.Replace(validManifest, "version: repoctx.manifest/v1alpha1", "version: 1", 1),
		"missing provider capability": strings.Replace(validManifest, "capabilities: [syntax_graph, artifact_declarations, go_imports]", "capabilities: [syntax_graph, artifact_declarations]", 1),
	}
	root := t.TempDir()
	write(t, root, "claims.json")
	write(t, root, "go.mod")
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			doc, err := Load(root, []byte(data), Limits{})
			if err == nil || doc != nil || len(err.Error()) > 256 {
				t.Fatalf("accepted invalid manifest: doc=%+v err=%v", doc, err)
			}
		})
	}
}

func TestLimitsAndMultipleDocuments(t *testing.T) {
	root := t.TempDir()
	write(t, root, "claims.json")
	write(t, root, "go.mod")
	tests := []struct {
		data   string
		limits Limits
		reason string
	}{
		{validManifest, Limits{Bytes: 10}, "byte limit"},
		{validManifest, Limits{Depth: 3}, "depth limit"},
		{validManifest, Limits{Entries: 5}, "entry limit"},
		{validManifest, Limits{Capabilities: 1}, "count limit"},
		{validManifest + "---\n{}\n", Limits{}, "multiple YAML documents"},
		{validManifest, Limits{Depth: 13}, "invalid caller limit"},
	}
	for _, test := range tests {
		if doc, err := Load(root, []byte(test.data), test.limits); err == nil || doc != nil ||
			!strings.Contains(err.Error(), test.reason) {
			t.Fatalf("reason %q: doc=%+v err=%v", test.reason, doc, err)
		}
	}
}

func TestRepositoryManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "agent-context.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, data, Limits{}); err != nil {
		t.Fatal(err)
	}
}

func FuzzLoad(f *testing.F) {
	root := f.TempDir()
	write(f, root, "claims.json")
	write(f, root, "go.mod")
	f.Add([]byte(validManifest))
	f.Add([]byte("a: &a [*a]\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Load(root, data, Limits{})
		if err != nil && (doc != nil || len(err.Error()) > 256) {
			t.Fatal("partial result or unbounded diagnostic")
		}
	})
}

func write(t testing.TB, root, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte("test\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
