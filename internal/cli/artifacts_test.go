package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactsCommandWritesAndChecksGeneratedCatalog(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	authoring := filepath.Join(root, "authoring.json")
	out := filepath.Join(root, "artifacts.json")
	if err := os.WriteFile(source, []byte("unique evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc := []byte(`{"version":"repoctx.artifact-authoring/v1alpha1","namespace":"test","source_anchors":[{"id":"sample","path":"source.txt","start":"unique evidence","end":"unique evidence"}],"artifacts":[{"id":"test:component.sample","kind":"component","applies_to":[{"kind":"file","path":"source.txt"}],"lifecycle":"active","sources":["sample"],"declared_inputs":[]}],"relationships":[]}`)
	if err := os.WriteFile(authoring, doc, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-root", root, "-source", authoring, "-o", out}
	if err := runArtifacts(args); err != nil {
		t.Fatal(err)
	}
	if err := runArtifacts(append(args, "-check")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("changed unique evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := runArtifacts(append(args, "-check")); err == nil {
		t.Fatal("stale generated catalog passed check")
	}
}

func TestArtifactsCommandRejectsInvalidUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"-source", "input.json", "-check"}, {"-unknown"}} {
		var usage *artifactsUsageError
		if err := runArtifacts(args); !errors.As(err, &usage) {
			t.Fatalf("runArtifacts(%v) error = %v; want usage error", args, err)
		}
	}
}
