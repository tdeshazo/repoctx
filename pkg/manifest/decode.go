package manifest

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"gopkg.in/yaml.v3"
)

var (
	idPattern        = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	providerPattern  = regexp.MustCompile(`^repoctx\.[a-z][a-z0-9.-]{0,95}/v[0-9]+$`)
)

type shape struct {
	kind     yaml.Kind
	required []string
	fields   map[string]*shape
	element  *shape
}

var (
	stringShape  = &shape{kind: yaml.ScalarNode}
	stringsShape = &shape{kind: yaml.SequenceNode, element: stringShape}
	rootShape    = object([]string{"id", "path"}, map[string]*shape{
		"id": stringShape, "path": stringShape,
	})
	documentShape = object([]string{"id", "path", "format"}, map[string]*shape{
		"id": stringShape, "path": stringShape, "format": stringShape,
	})
	artifactShape = object([]string{"id", "path", "format"}, map[string]*shape{
		"id": stringShape, "path": stringShape, "format": stringShape,
	})
	scopeShape = object([]string{"kind", "path"}, map[string]*shape{
		"kind": stringShape, "path": stringShape,
	})
	freshnessShape = object([]string{"inputs"}, map[string]*shape{"inputs": stringsShape})
	componentShape = object(
		[]string{"id", "source_roots", "artifact_sources", "provider_inputs", "owners", "scopes", "lifecycle", "supersedes", "sensitivity", "freshness"},
		map[string]*shape{"id": stringShape, "source_roots": stringsShape,
			"artifact_sources": stringsShape, "provider_inputs": stringsShape,
			"owners": stringsShape, "scopes": {kind: yaml.SequenceNode, element: scopeShape},
			"lifecycle": stringShape, "supersedes": stringsShape,
			"sensitivity": stringShape, "freshness": freshnessShape},
	)
	providerShape = object([]string{"id", "provider", "path"}, map[string]*shape{
		"id": stringShape, "provider": stringShape, "path": stringShape,
	})
	viewShape = object([]string{"id", "kind", "components"}, map[string]*shape{
		"id": stringShape, "kind": stringShape, "components": stringsShape,
	})
	manifestShape = object(
		[]string{"version", "namespace", "source_roots", "document_sources", "artifact_sources", "components", "provider_inputs", "derived_views", "capabilities"},
		map[string]*shape{
			"version":          stringShape,
			"namespace":        stringShape,
			"source_roots":     {kind: yaml.SequenceNode, element: rootShape},
			"document_sources": {kind: yaml.SequenceNode, element: documentShape},
			"artifact_sources": {kind: yaml.SequenceNode, element: artifactShape},
			"components":       {kind: yaml.SequenceNode, element: componentShape},
			"provider_inputs":  {kind: yaml.SequenceNode, element: providerShape},
			"derived_views":    {kind: yaml.SequenceNode, element: viewShape},
			"capabilities":     stringsShape,
		},
	)
)

func object(required []string, fields map[string]*shape) *shape {
	return &shape{kind: yaml.MappingNode, required: required, fields: fields}
}

// Load validates one manifest and verifies every declared path beneath root.
// Availability checks reject symlinks and non-regular file inputs. No provider
// is invoked and the manifest cannot widen the caller-selected root.
func Load(root string, data []byte, limits Limits) (*Manifest, error) {
	doc, _, normalized, err := decode(data, limits)
	if err != nil {
		return nil, err
	}
	if err := validate(doc, normalized); err != nil {
		return nil, err
	}
	if err := available(root, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// LoadFile reads a repository-relative manifest without following symlinks,
// then applies Load using the same caller-selected root.
func LoadFile(root, path string, limits Limits) (*Manifest, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	if !validPath(path, false) {
		return nil, fail("$", "invalid manifest path")
	}
	sources, err := sourceroot.Open(root)
	if err != nil {
		return nil, fail("$", "repository root is unavailable")
	}
	defer sources.Close()
	data, err := sources.Read(path, int64(normalized.Bytes))
	if err != nil {
		return nil, fail("$", "manifest is unavailable")
	}
	return Load(root, data, normalized)
}

func decode(data []byte, limits Limits) (*Manifest, *yaml.Node, Limits, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, nil, limits, err
	}
	if len(data) > normalized.Bytes {
		return nil, nil, limits, fail("$", "byte limit exceeded")
	}
	if !utf8.Valid(data) {
		return nil, nil, limits, fail("$", "invalid UTF-8")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil || len(node.Content) != 1 {
		return nil, nil, limits, fail("$", "invalid YAML syntax")
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, nil, limits, fail("$", "multiple YAML documents are not allowed")
	}
	entries := 0
	if err := checkNode(node.Content[0], manifestShape, "$", 1, normalized, &entries); err != nil {
		return nil, nil, limits, err
	}
	var doc Manifest
	if err := node.Content[0].Decode(&doc); err != nil {
		return nil, nil, limits, fail("$", "invalid field type")
	}
	return &doc, node.Content[0], normalized, nil
}

func checkNode(node *yaml.Node, expected *shape, path string, depth int, limits Limits, entries *int) error {
	if node.Alias != nil || node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fail(path, "YAML aliases and anchors are not allowed")
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return fail(path, "explicit YAML tags are not allowed")
	}
	if depth > limits.Depth {
		return fail(path, "depth limit exceeded")
	}
	*entries++
	if *entries > limits.Entries {
		return fail("$", "entry limit exceeded")
	}
	if node.Kind != expected.kind {
		return fail(path, "invalid field type")
	}
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" {
			return fail(path, "expected string")
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if err := checkNode(child, expected.element, fmt.Sprintf("%s[%d]", path, i), depth+1, limits, entries); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := make(map[string]bool, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			*entries++
			if *entries > limits.Entries {
				return fail("$", "entry limit exceeded")
			}
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || key.Style&yaml.TaggedStyle != 0 {
				return fail(path, "invalid field name")
			}
			name := key.Value
			if seen[name] {
				return fail(path, "duplicate field")
			}
			seen[name] = true
			field, ok := expected.fields[name]
			if !ok {
				return fail(path, "unknown field")
			}
			if err := checkNode(value, field, path+"."+name, depth+1, limits, entries); err != nil {
				return err
			}
		}
		for _, name := range expected.required {
			if !seen[name] {
				return fail(path+"."+name, "required field missing")
			}
		}
	}
	return nil
}

func normalizeLimits(limits Limits) (Limits, error) {
	ceiling := ceilings()
	pairs := [][2]*int{{&limits.Bytes, &ceiling.Bytes}, {&limits.Depth, &ceiling.Depth},
		{&limits.Entries, &ceiling.Entries}, {&limits.SourceRoots, &ceiling.SourceRoots},
		{&limits.DocumentSources, &ceiling.DocumentSources},
		{&limits.ArtifactSources, &ceiling.ArtifactSources}, {&limits.Components, &ceiling.Components},
		{&limits.ProviderInputs, &ceiling.ProviderInputs}, {&limits.DerivedViews, &ceiling.DerivedViews},
		{&limits.Capabilities, &ceiling.Capabilities}}
	for _, pair := range pairs {
		if *pair[0] < 0 || *pair[0] > *pair[1] {
			return limits, fail("$", "invalid caller limit")
		}
		if *pair[0] == 0 {
			*pair[0] = *pair[1]
		}
	}
	return limits, nil
}

func available(root string, doc *Manifest) error {
	sources, err := sourceroot.Open(root)
	if err != nil {
		return fail("$", "repository root is unavailable")
	}
	defer sources.Close()
	for i, source := range doc.SourceRoots {
		if _, err := sources.ReadDir(source.Path, 1); err != nil && err != io.EOF {
			return fail(fmt.Sprintf("$.source_roots[%d].path", i), "source root is unavailable")
		}
	}
	for i, source := range doc.ArtifactSources {
		if _, err := sources.Read(source.Path, 2<<20); err != nil {
			return fail(fmt.Sprintf("$.artifact_sources[%d].path", i), "artifact source is unavailable")
		}
	}
	for i, source := range doc.DocumentSources {
		if _, err := sources.Read(source.Path, 2<<20); err != nil {
			return fail(fmt.Sprintf("$.document_sources[%d].path", i), "document source is unavailable")
		}
	}
	for i, input := range doc.ProviderInputs {
		if _, err := sources.Read(input.Path, 2<<20); err != nil {
			return fail(fmt.Sprintf("$.provider_inputs[%d].path", i), "provider input is unavailable")
		}
	}
	return nil
}

func fail(path, reason string) error { return fmt.Errorf("%s: %s", path, reason) }

func validPath(value string, root bool) bool {
	if len(value) == 0 || len(value) > 4096 || !utf8.ValidString(value) {
		return false
	}
	if root && value == "." {
		return true
	}
	for _, r := range value {
		if r < 32 || r == 127 || strings.ContainsRune(`\:*?[]{}!`, r) {
			return false
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
