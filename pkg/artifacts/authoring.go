package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
)

// AuthoringVersion identifies the compact artifact-authoring contract. Its
// source anchors are resolved into exact artifact wire spans by Generate.
const AuthoringVersion = "repoctx.artifact-authoring/v1alpha2"

// PathPermit decides whether canonical compilation may read an anchored path.
// A nil permit allows every path beneath the already caller-selected root.
type PathPermit func(path string) bool

// Compilation is the single validated result used by both the canonical entity
// adapter and the deterministic standalone catalog projection.
type Compilation struct {
	Authoring *AuthoringCatalog
	Catalog   *Catalog
}

// AuthoringCatalog contains semantic declarations without derived provenance.
type AuthoringCatalog struct {
	Version       string                  `json:"version"`
	Namespace     string                  `json:"namespace"`
	SourceAnchors []NamedSourceAnchor     `json:"source_anchors"`
	Artifacts     []AuthoringArtifact     `json:"artifacts"`
	Relationships []AuthoringRelationship `json:"relationships"`
}

// AuthoringArtifact is an artifact declaration with source anchors in place of
// hashes and physical coordinates.
type AuthoringArtifact struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Owner          *string         `json:"owner,omitempty"`
	AppliesTo      []Applicability `json:"applies_to"`
	Lifecycle      string          `json:"lifecycle"`
	Sensitivity    string          `json:"sensitivity"`
	Sources        []string        `json:"sources"`
	DeclaredInputs []Input         `json:"declared_inputs"`
	RunnerCheckID  *string         `json:"runner_check_id,omitempty"`
}

// AuthoringRelationship is a relationship declaration with source anchors.
type AuthoringRelationship struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Kind       string   `json:"kind"`
	Resolution string   `json:"resolution"`
	Sources    []string `json:"sources"`
}

// NamedSourceAnchor selects the inclusive byte range from the unique Start
// text to the unique End text in Path. Equal anchors select one occurrence.
type NamedSourceAnchor struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// Generate strictly decodes an authoring catalog, resolves its anchors beneath
// root, and returns a deterministic, line-oriented artifact wire catalog.
func Generate(root string, data []byte, limits Limits) ([]byte, error) {
	compiled, err := Compile(root, data, limits, nil)
	if err != nil {
		return nil, err
	}
	out, err := encodeCatalog(compiled.Catalog)
	if err != nil {
		return nil, fmt.Errorf("encode artifact catalog: %w", err)
	}
	return out, nil
}

// Compile validates artifact authoring, resolves its exact source anchors, and
// returns both the canonical authoring declarations and derived catalog. It
// performs no provider activation, authority resolution, or command execution.
func Compile(root string, data []byte, limits Limits, permit PathPermit) (*Compilation, error) {
	doc, normalized, err := decodeAuthoring(data, limits)
	if err != nil {
		return nil, err
	}
	anchors, err := validateAuthoring(doc, normalized)
	if err != nil {
		return nil, err
	}
	catalog, err := authoringCatalog(doc, anchors, placeholderSpan)
	if err != nil {
		return nil, err
	}
	if err := validateGenerated(catalog, normalized); err != nil {
		return nil, err
	}

	sources, err := sourceroot.Open(root)
	if err != nil {
		return nil, fmt.Errorf("open source root: %w", err)
	}
	defer sources.Close()
	cache := make(map[string][]byte)
	resolved := make(map[string]Span, len(doc.SourceAnchors))
	for _, anchor := range doc.SourceAnchors {
		if permit != nil && !permit(anchor.Path) {
			return nil, fmt.Errorf("source anchor %q: path is outside caller scope", anchor.ID)
		}
		content, ok := cache[anchor.Path]
		if !ok {
			content, err = sources.Read(anchor.Path, 2<<20)
			if err != nil {
				return nil, fmt.Errorf("source anchor %q: read source %q: %w", anchor.ID, anchor.Path, err)
			}
			if !utf8.Valid(content) {
				return nil, fmt.Errorf("source anchor %q: source %q is not UTF-8", anchor.ID, anchor.Path)
			}
			cache[anchor.Path] = content
		}
		span, resolveErr := resolveAnchor(content, anchor)
		if resolveErr != nil {
			return nil, fmt.Errorf("source anchor %q: %w", anchor.ID, resolveErr)
		}
		resolved[anchor.ID] = span
	}
	catalog, err = authoringCatalog(doc, anchors, func(anchor NamedSourceAnchor) Span {
		return resolved[anchor.ID]
	})
	if err != nil {
		return nil, err
	}
	if err := validateGenerated(catalog, normalized); err != nil {
		return nil, err
	}
	return &Compilation{Authoring: doc, Catalog: catalog}, nil
}

func decodeAuthoring(data []byte, limits Limits) (*AuthoringCatalog, Limits, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, limits, err
	}
	if len(data) > normalized.Bytes {
		return nil, limits, failure("$", "byte limit exceeded")
	}
	if !utf8.Valid(data) {
		return nil, limits, failure("$", "invalid UTF-8")
	}
	if err := checkEscapes(data); err != nil {
		return nil, limits, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scan(decoder, 0, normalized.Depth); err != nil {
		return nil, limits, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, limits, failure("$", "trailing JSON or invalid syntax")
	}
	if err := checkShape(data, reflect.TypeOf(AuthoringCatalog{}), "$"); err != nil {
		return nil, limits, err
	}
	var doc AuthoringCatalog
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, limits, failure("$", "invalid field type")
	}
	if doc.Version != AuthoringVersion {
		return nil, limits, failure("$.version", "unsupported version")
	}
	return &doc, normalized, nil
}

func validateAuthoring(doc *AuthoringCatalog, limits Limits) (map[string]NamedSourceAnchor, error) {
	if doc.SourceAnchors == nil {
		return nil, failure("$.source_anchors", "required array missing")
	}
	if len(doc.SourceAnchors) > limits.NestedEntries {
		return nil, failure("$.source_anchors", "count limit exceeded")
	}
	anchors := make(map[string]NamedSourceAnchor, len(doc.SourceAnchors))
	for i, anchor := range doc.SourceAnchors {
		if !checkPattern.MatchString(anchor.ID) || len(anchor.ID) > 256 {
			return nil, fmt.Errorf("$.source_anchors[%d].id: invalid anchor ID", i)
		}
		if _, exists := anchors[anchor.ID]; exists {
			return nil, fmt.Errorf("$.source_anchors[%d].id: duplicate anchor ID", i)
		}
		if err := validateAnchor(anchor); err != nil {
			return nil, fmt.Errorf("$.source_anchors[%d]: %w", i, err)
		}
		anchors[anchor.ID] = anchor
	}
	for i, artifact := range doc.Artifacts {
		if !oneOf(artifact.Sensitivity, "public", "internal", "restricted") {
			return nil, fmt.Errorf("$.artifacts[%d].sensitivity: unknown sensitivity", i)
		}
		if artifact.Owner != nil && (!idPattern.MatchString(*artifact.Owner) || !strings.HasPrefix(*artifact.Owner, doc.Namespace+":")) {
			return nil, fmt.Errorf("$.artifacts[%d].owner: invalid local owner identity", i)
		}
	}
	return anchors, nil
}

func validateAnchor(anchor NamedSourceAnchor) error {
	if !validPath(anchor.Path, false) {
		return fmt.Errorf("invalid path")
	}
	if anchor.Start == "" || anchor.End == "" || len(anchor.Start) > 4096 || len(anchor.End) > 4096 ||
		!utf8.ValidString(anchor.Start) || !utf8.ValidString(anchor.End) {
		return fmt.Errorf("anchors must be nonempty UTF-8 of at most 4096 bytes")
	}
	return nil
}

func authoringCatalog(
	doc *AuthoringCatalog,
	anchors map[string]NamedSourceAnchor,
	makeSpan func(NamedSourceAnchor) Span,
) (*Catalog, error) {
	catalog := &Catalog{Version: Version, Namespace: doc.Namespace}
	used := make(map[string]bool, len(anchors))
	catalog.Artifacts = make([]Artifact, len(doc.Artifacts))
	for i, source := range doc.Artifacts {
		artifact := Artifact{ID: source.ID, Kind: source.Kind, Owner: source.Owner,
			AppliesTo: source.AppliesTo, Lifecycle: source.Lifecycle,
			DeclaredInputs: source.DeclaredInputs, RunnerCheckID: source.RunnerCheckID}
		artifact.Sources = make([]Span, len(source.Sources))
		for j, sourceID := range source.Sources {
			anchor, ok := anchors[sourceID]
			if !ok {
				return nil, fmt.Errorf("$.artifacts[%d].sources[%d]: unknown source anchor", i, j)
			}
			used[sourceID] = true
			artifact.Sources[j] = makeSpan(anchor)
		}
		catalog.Artifacts[i] = artifact
	}
	catalog.Relationships = make([]Relationship, len(doc.Relationships))
	for i, source := range doc.Relationships {
		relationship := Relationship{From: source.From, To: source.To, Kind: source.Kind,
			Resolution: source.Resolution, Sources: make([]Span, len(source.Sources))}
		for j, sourceID := range source.Sources {
			anchor, ok := anchors[sourceID]
			if !ok {
				return nil, fmt.Errorf("$.relationships[%d].sources[%d]: unknown source anchor", i, j)
			}
			used[sourceID] = true
			relationship.Sources[j] = makeSpan(anchor)
		}
		catalog.Relationships[i] = relationship
	}
	for i, anchor := range doc.SourceAnchors {
		if !used[anchor.ID] {
			return nil, fmt.Errorf("$.source_anchors[%d]: unused source anchor", i)
		}
	}
	return catalog, nil
}

func placeholderSpan(anchor NamedSourceAnchor) Span {
	return Span{Path: anchor.Path, SHA256: string(bytes.Repeat([]byte{'0'}, 64)),
		StartByte: 0, EndByte: 1, StartLine: 1, EndLine: 1, EndByteColumn: 1}
}

func validateGenerated(catalog *Catalog, limits Limits) error {
	wire, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	_, err = Decode(wire, limits)
	return err
}

func encodeCatalog(catalog *Catalog) ([]byte, error) {
	var out bytes.Buffer
	version, err := json.Marshal(catalog.Version)
	if err != nil {
		return nil, err
	}
	namespace, err := json.Marshal(catalog.Namespace)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&out, "{\n  \"version\": %s,\n  \"namespace\": %s,\n  \"artifacts\": [\n", version, namespace)
	for i, artifact := range catalog.Artifacts {
		encoded, marshalErr := json.Marshal(artifact)
		if marshalErr != nil {
			return nil, marshalErr
		}
		fmt.Fprintf(&out, "    %s", encoded)
		if i+1 < len(catalog.Artifacts) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("  ],\n  \"relationships\": [\n")
	for i, relationship := range catalog.Relationships {
		encoded, marshalErr := json.Marshal(relationship)
		if marshalErr != nil {
			return nil, marshalErr
		}
		fmt.Fprintf(&out, "    %s", encoded)
		if i+1 < len(catalog.Relationships) {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("  ]\n}\n")
	return out.Bytes(), nil
}

func resolveAnchor(content []byte, anchor NamedSourceAnchor) (Span, error) {
	startText, endText := []byte(anchor.Start), []byte(anchor.End)
	start, startUnique := uniqueIndex(content, startText)
	if !startUnique {
		return Span{}, fmt.Errorf("start anchor must occur exactly once")
	}
	endStart, endUnique := uniqueIndex(content, endText)
	if !endUnique {
		return Span{}, fmt.Errorf("end anchor must occur exactly once")
	}
	end := endStart + len(endText)
	if endStart < start || end < start+len(startText) {
		return Span{}, fmt.Errorf("end anchor precedes start anchor")
	}
	startLine, startColumn := physicalPosition(content, start)
	endLine, endColumn := physicalPosition(content, end)
	digest := sha256.Sum256(content)
	return Span{Path: anchor.Path, SHA256: fmt.Sprintf("%x", digest),
		StartByte: int64(start), EndByte: int64(end), StartLine: int64(startLine),
		EndLine: int64(endLine), StartByteColumn: int64(startColumn), EndByteColumn: int64(endColumn)}, nil
}

func uniqueIndex(content, anchor []byte) (int, bool) {
	first := bytes.Index(content, anchor)
	if first < 0 {
		return -1, false
	}
	return first, !bytes.Contains(content[first+1:], anchor)
}

func physicalPosition(data []byte, offset int) (line, column int) {
	line = 1
	for _, value := range data[:offset] {
		if value == '\n' {
			line, column = line+1, 0
		} else {
			column++
		}
	}
	return line, column
}
