package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/obligation"
)

func TestRepositoryArtifactCatalog(t *testing.T) {
	root := "."
	wire, err := os.ReadFile(filepath.Join(root, "docs/repoctx-artifacts.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, duplicatedInstruction := range [][]byte{[]byte("go test ./..."), []byte("go vet ./...")} {
		if bytes.Contains(wire, duplicatedInstruction) {
			t.Fatalf("catalog copied development command %q", duplicatedInstruction)
		}
	}
	catalog, err := artifacts.Decode(wire, artifacts.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	verifyCatalogSources(t, root, catalog)

	authority := artifacts.Authority{
		AcceptedIDs: artifactIDs(catalog),
		Scopes:      []artifacts.Applicability{{Kind: "subtree", Path: "."}},
	}
	resolution, err := artifacts.Resolve(catalog, authority)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolution.Diagnostics) != 0 || len(resolution.Artifacts) != len(catalog.Artifacts) {
		t.Fatalf("catalog did not resolve completely: %+v", resolution)
	}

	denyPaths := []string{"docs/reports", "dogfood_test.go"}
	repo, err := compiler.Compile(compiler.Options{Root: root, DenyPaths: denyPaths})
	if err != nil {
		t.Fatal(err)
	}
	grounding, err := artifacts.GroundRelationships(repo, catalog, artifacts.GroundOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if grounding.Incomplete || len(grounding.Diagnostics) != 0 ||
		len(grounding.Relationships) != len(catalog.Relationships) {
		t.Fatalf("catalog links did not ground completely: %+v", grounding)
	}

	milestone, gate := nextRoadmapPair(t, root, catalog)
	initial, err := agentctx.Build(repo, agentctx.Options{
		Root: root, Query: "next outstanding roadmap item", Depth: 0, MaxBytes: 20000,
		DenyPaths: denyPaths,
	})
	if err != nil {
		t.Fatal(err)
	}
	unitID := coveringUnit(t, initial.Bundle, milestone.Sources[0], gate.Sources[0])
	expanded, err := agentctx.Build(repo, agentctx.Options{
		Root: root, Units: []string{unitID}, ExpectedSnapshot: initial.Bundle.Snapshot.ID,
		MaxBytes: 96000, DenyPaths: denyPaths,
	})
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := obligation.Build(expanded, catalog, authority, obligation.Options{MaxBytes: 48000})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{milestone.ID, gate.ID} {
		artifact := selectedArtifact(t, handoff.Bundle, id)
		if len(artifact.Sources) == 0 || artifact.Sources[0].Verification != "context_exact" {
			t.Fatalf("%s lacks exact expanded evidence: %+v", id, artifact.Sources)
		}
	}
}

func verifyCatalogSources(t *testing.T, root string, catalog *artifacts.Catalog) {
	t.Helper()
	contents := map[string][]byte{}
	verify := func(span artifacts.Span) {
		data, ok := contents[span.Path]
		if !ok {
			var err error
			data, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(span.Path)))
			if err != nil {
				t.Fatal(err)
			}
			contents[span.Path] = data
		}
		digest := sha256.Sum256(data)
		if fmt.Sprintf("%x", digest) != span.SHA256 {
			t.Fatalf("stale catalog hash for %s", span.Path)
		}
		if span.StartByte < 0 || span.EndByte > int64(len(data)) || span.StartByte >= span.EndByte {
			t.Fatalf("invalid catalog byte range for %s", span.Path)
		}
		if !utf8.Valid(data[span.StartByte:span.EndByte]) {
			t.Fatalf("catalog span splits utf-8 in %s", span.Path)
		}
		startLine, startColumn := physicalPosition(data, int(span.StartByte))
		endLine, endColumn := physicalPosition(data, int(span.EndByte))
		if int64(startLine) != span.StartLine || int64(startColumn) != span.StartByteColumn ||
			int64(endLine) != span.EndLine || int64(endColumn) != span.EndByteColumn {
			t.Fatalf("stale catalog coordinates for %s", span.Path)
		}
	}
	for _, artifact := range catalog.Artifacts {
		for _, span := range artifact.Sources {
			verify(span)
		}
	}
	for _, relationship := range catalog.Relationships {
		for _, span := range relationship.Sources {
			verify(span)
		}
	}
}

func physicalPosition(data []byte, offset int) (int, int) {
	line, column := 1, 0
	for _, value := range data[:offset] {
		if value == '\n' {
			line, column = line+1, 0
			continue
		}
		column++
	}
	return line, column
}

func artifactIDs(catalog *artifacts.Catalog) []string {
	ids := make([]string, 0, len(catalog.Artifacts))
	for _, artifact := range catalog.Artifacts {
		ids = append(ids, artifact.ID)
	}
	return ids
}

func nextRoadmapPair(t *testing.T, root string, catalog *artifacts.Catalog) (artifacts.Artifact, artifacts.Artifact) {
	t.Helper()
	byID := make(map[string]artifacts.Artifact, len(catalog.Artifacts))
	for _, artifact := range catalog.Artifacts {
		byID[artifact.ID] = artifact
	}
	for _, relationship := range catalog.Relationships {
		from, fromOK := byID[relationship.From]
		to, toOK := byID[relationship.To]
		isRoadmapPair := relationship.Kind == "contains" && fromOK && toOK &&
			from.Kind == "component" && to.Kind == "requirement" &&
			len(from.Sources) == 1 && len(to.Sources) == 1 &&
			from.Sources[0].Path == "ROADMAP.md" && to.Sources[0].Path == "ROADMAP.md"
		if !isRoadmapPair {
			continue
		}
		roadmap, err := os.ReadFile(filepath.Join(root, "ROADMAP.md"))
		if err != nil {
			t.Fatal(err)
		}
		gateStart := int(to.Sources[0].StartByte)
		gateText := roadmap[gateStart:to.Sources[0].EndByte]
		if !strings.HasPrefix(string(gateText), "- [ ] **M") {
			t.Fatalf("declared next gate is not incomplete: %q", gateText)
		}
		if strings.Contains(string(roadmap[:gateStart]), "- [ ] **M") {
			t.Fatal("catalog skipped an earlier incomplete roadmap item")
		}
		return from, to
	}
	t.Fatal("catalog has no incomplete milestone and next-gate relationship")
	return artifacts.Artifact{}, artifacts.Artifact{}
}

func coveringUnit(
	t *testing.T,
	bundle *agentctx.Bundle,
	milestone artifacts.Span,
	gate artifacts.Span,
) string {
	t.Helper()
	bestID, bestLines := "", int(^uint(0)>>1)
	for _, unit := range bundle.Units {
		if unit.File != milestone.Path || unit.File != gate.Path {
			continue
		}
		coversMilestone := unit.Extent.StartLine <= int(milestone.StartLine) &&
			unit.Extent.EndLine >= int(milestone.EndLine)
		coversGate := unit.Extent.StartLine <= int(gate.StartLine) &&
			unit.Extent.EndLine >= int(gate.EndLine)
		lines := unit.Extent.EndLine - unit.Extent.StartLine
		if coversMilestone && coversGate && lines < bestLines {
			bestID, bestLines = unit.ID, lines
		}
	}
	if bestID == "" {
		t.Fatal("initial retrieval did not expose a unit covering the roadmap pair")
	}
	return bestID
}

func selectedArtifact(t *testing.T, bundle *obligation.Bundle, id string) obligation.Artifact {
	t.Helper()
	for _, artifact := range bundle.Artifacts {
		if artifact.ID == id {
			return artifact
		}
	}
	t.Fatalf("handoff omitted %s", id)
	return obligation.Artifact{}
}
