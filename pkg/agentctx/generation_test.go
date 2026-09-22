package agentctx

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestVerifiedSourceGenerationReusesOwnedSnapshot(t *testing.T) {
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if generation.SnapshotID() == "" || generation.SourceID() == "" || generation.ProfileID() == "" {
		t.Fatal("generation identities are incomplete")
	}
	options := baseOptions("")
	options.ExpectedSnapshot = generation.SnapshotID()
	first, err := BuildFromGeneration(generation, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bundle.Snapshot.Verification != "immutable" ||
		first.Bundle.Snapshot.SourceID != generation.SourceID() ||
		!strings.Contains(strings.Join(first.Bundle.Warnings, " "), "in-memory generation") {
		t.Fatal("generation provenance is not explicit")
	}
	strict, err := Build(repo, baseOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Bundle.Symbols, strict.Bundle.Symbols) ||
		!reflect.DeepEqual(first.Bundle.Units, strict.Bundle.Units) ||
		!reflect.DeepEqual(first.Bundle.Evidence, strict.Bundle.Evidence) ||
		!reflect.DeepEqual(first.Bundle.Relationships, strict.Bundle.Relationships) {
		t.Fatal("generation reuse changed selected semantic evidence")
	}
	immutableOptions := baseOptions(root)
	immutableOptions.Consistency = "immutable"
	immutableOptions.ExpectedSnapshot = generation.SnapshotID()
	immutable, err := Build(repo, immutableOptions)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bundle.TaskID == immutable.Bundle.TaskID {
		t.Fatal("generation provenance omitted from task identity")
	}
	assertEvidence(t, root, first.Bundle)

	// Neither later filesystem changes nor mutation of the caller's original
	// index can alter bytes privately owned by the verified generation.
	write(t, root, "main.go", "package changed\nfunc Ping() {}\n")
	write(t, root, "added.md", "# Added\n")
	repo.Strings[0] = "mutated by caller"
	again, err := BuildFromGeneration(generation, options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Payload, again.Payload) {
		t.Fatal("owned generation changed after caller or filesystem mutation")
	}
	if _, err := Build(repo, baseOptions(root)); err == nil {
		t.Fatal("strict local build accepted mutated index or sources")
	}
}

func TestVerifiedSourceGenerationRequiresAuthenticatedBinding(t *testing.T) {
	root, repo := goFixture(t)
	if _, err := VerifySourceGeneration(nil, SourceGenerationOptions{Root: root}); err == nil {
		t.Fatal("nil index accepted")
	}
	if _, err := VerifySourceGeneration(repo, SourceGenerationOptions{}); err == nil {
		t.Fatal("missing root accepted")
	}
	if _, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root, MaxIndexBytes: 1}); err == nil {
		t.Fatal("index byte limit ignored")
	}
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildFromGeneration(nil, Options{}); err == nil {
		t.Fatal("nil generation accepted")
	}
	options := baseOptions("")
	if _, err := BuildFromGeneration(generation, options); err == nil {
		t.Fatal("unbound generation accepted")
	}
	options.ExpectedSnapshot = "sha256:" + strings.Repeat("0", 64)
	if _, err := BuildFromGeneration(generation, options); err == nil {
		t.Fatal("wrong authenticated snapshot accepted")
	}
	options.ExpectedSnapshot = generation.SnapshotID()
	options.Consistency = "verified-local"
	if _, err := BuildFromGeneration(generation, options); err == nil {
		t.Fatal("contradictory consistency mode accepted")
	}
	options.Consistency = ""
	options.AllowPaths = []string{"different"}
	if _, err := BuildFromGeneration(generation, options); err == nil {
		t.Fatal("incompatible authorization scope accepted")
	}
}

func TestVerifySourceGenerationRejectsStaleSources(t *testing.T) {
	root, repo := goFixture(t)
	write(t, root, "main.go", "package changed\n")
	if _, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root}); err == nil {
		t.Fatal("stale source generation accepted")
	}
}
