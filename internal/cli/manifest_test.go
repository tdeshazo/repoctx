package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/manifest"
)

func TestRunManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	var output bytes.Buffer
	if err := runManifest([]string{"-root", root, "-file", "agent-context.yaml"}, &output); err != nil {
		t.Fatal(err)
	}
	var doc manifest.Manifest
	if err := json.Unmarshal(output.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != manifest.Version || len(doc.Components) == 0 {
		t.Fatalf("unexpected manifest output: %+v", doc)
	}
}

func TestRunManifestRejectsUnavailableInput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "agent-context.yaml")
	data := `version: repoctx.manifest/v1alpha1
namespace: demo
source_roots: [{id: source, path: .}]
artifact_sources: []
components:
  - id: core
    source_roots: [source]
    artifact_sources: []
    provider_inputs: [missing]
provider_inputs:
  - id: missing
    provider: repoctx.go-imports/v1
    path: absent
derived_views: []
capabilities: []
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runManifest([]string{"-root", root, "-file", path}, &bytes.Buffer{}); err == nil {
		t.Fatal("unavailable input accepted")
	}
}
