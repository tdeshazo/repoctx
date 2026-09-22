package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/ir"
	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Version  string              `yaml:"version"`
	Entities []entityDeclaration `yaml:"entities"`
}

type entityDeclaration struct {
	ID          string    `yaml:"id"`
	Kind        string    `yaml:"kind"`
	Owners      []string  `yaml:"owners"`
	Scopes      []Scope   `yaml:"scopes"`
	Lifecycle   string    `yaml:"lifecycle"`
	Supersedes  []string  `yaml:"supersedes"`
	Sensitivity string    `yaml:"sensitivity"`
	Freshness   Freshness `yaml:"freshness"`
}

var (
	entityShape = object(
		[]string{"id", "kind", "owners", "scopes", "lifecycle", "supersedes", "sensitivity", "freshness"},
		map[string]*shape{
			"id": stringShape, "kind": stringShape, "owners": stringsShape,
			"scopes":    {kind: yaml.SequenceNode, element: scopeShape},
			"lifecycle": stringShape, "supersedes": stringsShape,
			"sensitivity": stringShape, "freshness": freshnessShape,
		},
	)
	frontmatterShape = object(
		[]string{"version", "entities"},
		map[string]*shape{
			"version":  stringShape,
			"entities": {kind: yaml.SequenceNode, element: entityShape},
		},
	)
)

// CompileEntities compiles manifest components, artifact authoring, and
// explicitly listed Markdown frontmatter into stable, source-linked repository
// entities. It validates claims but grants no authority and invokes no provider.
func CompileEntities(root, path string, limits Limits, permit func(string) bool) (*ir.EntityModel, error) {
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	if !validPath(path, false) {
		return nil, fail("$", "invalid manifest path")
	}
	if permit != nil && !permit(path) {
		return nil, fail("$", "manifest is outside caller scope")
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
	doc, node, normalized, err := decode(data, normalized)
	if err != nil {
		return nil, err
	}
	if err := validate(doc, normalized); err != nil {
		return nil, err
	}
	if err := permitted(doc, permit); err != nil {
		return nil, err
	}
	if err := available(root, doc); err != nil {
		return nil, err
	}

	model := &ir.EntityModel{
		Version: ir.EntityVersion, Namespace: doc.Namespace,
		Entities: []ir.Entity{}, Relationships: []ir.EntityRelationship{},
	}
	componentNodes := sequenceMappings(node, "components")
	for i, component := range doc.Components {
		if i >= len(componentNodes) {
			return nil, fail("$.components", "source mapping unavailable")
		}
		model.Entities = append(model.Entities, componentEntity(doc.Namespace, path, data, componentNodes[i], component))
	}
	for i, source := range doc.ArtifactSources {
		authoring, err := sources.Read(source.Path, 1<<20)
		if err != nil {
			return nil, fail(fmt.Sprintf("$.artifact_sources[%d].path", i), "artifact source is unavailable")
		}
		fragment, err := artifacts.Entities(root, authoring, permit)
		if err != nil {
			return nil, fail(fmt.Sprintf("$.artifact_sources[%d]", i), err.Error())
		}
		if fragment.Namespace != doc.Namespace {
			return nil, fail(fmt.Sprintf("$.artifact_sources[%d]", i), "namespace disagrees with manifest")
		}
		model.Entities = append(model.Entities, fragment.Entities...)
		model.Relationships = append(model.Relationships, fragment.Relationships...)
	}
	for i, source := range doc.DocumentSources {
		documentData, err := sources.Read(source.Path, 2<<20)
		if err != nil {
			return nil, fail(fmt.Sprintf("$.document_sources[%d].path", i), "document source is unavailable")
		}
		entities, err := decodeFrontmatter(doc.Namespace, source.Path, documentData, normalized)
		if err != nil {
			return nil, fail(fmt.Sprintf("$.document_sources[%d]", i), err.Error())
		}
		model.Entities = append(model.Entities, entities...)
	}
	if len(model.Entities) > 1024 {
		return nil, fail("$.entities", "count limit exceeded")
	}
	if len(model.Relationships) > 4096 {
		return nil, fail("$.relationships", "count limit exceeded")
	}
	slices.SortFunc(model.Entities, func(a, b ir.Entity) int { return strings.Compare(a.ID, b.ID) })
	slices.SortFunc(model.Relationships, compareRelationships)
	if err := validateEntities(model); err != nil {
		return nil, err
	}
	return model, nil
}

func compareRelationships(a, b ir.EntityRelationship) int {
	if result := strings.Compare(a.From, b.From); result != 0 {
		return result
	}
	if result := strings.Compare(a.Kind, b.Kind); result != 0 {
		return result
	}
	return strings.Compare(a.To, b.To)
}

func componentEntity(namespace, path string, data []byte, node *yaml.Node, component Component) ir.Entity {
	return ir.Entity{
		ID: qualify(namespace, component.ID), Kind: "component",
		Owners: qualifyAll(namespace, component.Owners), Scopes: irScopes(component.Scopes),
		Lifecycle: component.Lifecycle, Supersedes: qualifyAll(namespace, component.Supersedes),
		Sensitivity: component.Sensitivity, Freshness: ir.Freshness{Inputs: slices.Clone(component.Freshness.Inputs)},
		Declaration: ir.Declaration{Status: "declared", Sources: []ir.SourceSpan{sourceSpan(path, data, node, 0)}},
	}
}

func decodeFrontmatter(namespace, path string, data []byte, limits Limits) ([]ir.Entity, error) {
	content, lineOffset, err := frontmatterBytes(data)
	if err != nil {
		return nil, err
	}
	if len(content) > limits.Bytes || !utf8.Valid(content) {
		return nil, fmt.Errorf("frontmatter exceeds limits")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil || len(document.Content) != 1 {
		return nil, fmt.Errorf("invalid frontmatter syntax")
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("multiple frontmatter documents are not allowed")
	}
	entries := 0
	root := document.Content[0]
	if err := checkNode(root, frontmatterShape, "$", 1, limits, &entries); err != nil {
		return nil, err
	}
	var authored frontmatter
	if err := root.Decode(&authored); err != nil {
		return nil, fmt.Errorf("invalid frontmatter field type")
	}
	if authored.Version != FrontmatterVersion {
		return nil, fmt.Errorf("unsupported frontmatter version")
	}
	if len(authored.Entities) == 0 {
		return nil, fmt.Errorf("at least one entity is required")
	}
	if len(authored.Entities) > 1024 {
		return nil, fmt.Errorf("entity count limit exceeded")
	}
	nodes := sequenceMappings(root, "entities")
	if len(nodes) != len(authored.Entities) {
		return nil, fmt.Errorf("entity source mapping unavailable")
	}
	result := make([]ir.Entity, 0, len(authored.Entities))
	for i, declaration := range authored.Entities {
		if err := validateDeclaration(declaration, fmt.Sprintf("$.entities[%d]", i)); err != nil {
			return nil, err
		}
		result = append(result, ir.Entity{
			ID: qualify(namespace, declaration.ID), Kind: declaration.Kind,
			Owners: qualifyAll(namespace, declaration.Owners), Scopes: irScopes(declaration.Scopes),
			Lifecycle: declaration.Lifecycle, Supersedes: qualifyAll(namespace, declaration.Supersedes),
			Sensitivity: declaration.Sensitivity,
			Freshness:   ir.Freshness{Inputs: slices.Clone(declaration.Freshness.Inputs)},
			Declaration: ir.Declaration{Status: "declared", Sources: []ir.SourceSpan{sourceSpan(path, data, nodes[i], lineOffset)}},
		})
	}
	return result, nil
}

func validateDeclaration(entity entityDeclaration, path string) error {
	if !idPattern.MatchString(entity.ID) {
		return fail(path+".id", "invalid identity")
	}
	if !oneOf(entity.Kind, "document", "component", "decision", "contract", "requirement", "verification_obligation", "owner") {
		return fail(path+".kind", "unsupported entity kind")
	}
	if entity.Owners == nil || entity.Scopes == nil || entity.Supersedes == nil || entity.Freshness.Inputs == nil {
		return fail(path, "required array missing")
	}
	if err := localReferences(entity.Owners, path+".owners"); err != nil {
		return err
	}
	if err := scopes(entity.Scopes, path+".scopes"); err != nil {
		return err
	}
	if !oneOf(entity.Lifecycle, "draft", "active", "deprecated", "superseded", "retired") {
		return fail(path+".lifecycle", "unsupported lifecycle")
	}
	if err := localReferences(entity.Supersedes, path+".supersedes"); err != nil {
		return err
	}
	if !oneOf(entity.Sensitivity, "public", "internal", "restricted") {
		return fail(path+".sensitivity", "unsupported sensitivity")
	}
	return inputPaths(entity.Freshness.Inputs, path+".freshness.inputs")
}

func validateEntities(model *ir.EntityModel) error {
	byID := make(map[string]ir.Entity, len(model.Entities))
	for i, entity := range model.Entities {
		if _, exists := byID[entity.ID]; exists {
			return fail(fmt.Sprintf("$.entities[%d].id", i), "duplicate entity identity")
		}
		byID[entity.ID] = entity
	}
	for i, entity := range model.Entities {
		for j, owner := range entity.Owners {
			target, exists := byID[owner]
			if !exists || target.Kind != "owner" {
				return fail(fmt.Sprintf("$.entities[%d].owners[%d]", i, j), "unknown owner entity")
			}
		}
		for j, prior := range entity.Supersedes {
			target, exists := byID[prior]
			if !exists || target.Kind != entity.Kind || target.ID == entity.ID {
				return fail(fmt.Sprintf("$.entities[%d].supersedes[%d]", i, j), "invalid supersession target")
			}
		}
	}
	return nil
}

func sequenceMappings(node *yaml.Node, field string) []*yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == field && node.Content[i+1].Kind == yaml.SequenceNode {
			return node.Content[i+1].Content
		}
	}
	return nil
}

func frontmatterBytes(data []byte) ([]byte, int, error) {
	offset := 0
	line := 1
	for _, part := range bytes.SplitAfter(data, []byte("\n")) {
		text := strings.TrimSuffix(strings.TrimSuffix(string(part), "\n"), "\r")
		if line == 1 && text != "---" {
			return nil, 0, fmt.Errorf("frontmatter must be first")
		}
		if line > 1 && text == "---" {
			start := len(firstLine(data))
			return data[start:offset], 1, nil
		}
		offset += len(part)
		line++
	}
	return nil, 0, fmt.Errorf("frontmatter closing delimiter missing")
}

func firstLine(data []byte) []byte {
	if index := bytes.IndexByte(data, '\n'); index >= 0 {
		return data[:index+1]
	}
	return data
}

func sourceSpan(path string, data []byte, node *yaml.Node, lineOffset int) ir.SourceSpan {
	startLine := node.Line + lineOffset
	endLine := maxNodeLine(node) + lineOffset
	starts := lineStarts(data)
	start := starts[startLine-1]
	end := len(data)
	if endLine < len(starts) {
		end = starts[endLine]
	}
	hash := sha256.Sum256(data)
	endPhysicalLine, endColumn := position(data, end)
	return ir.SourceSpan{
		Path: path, SHA256: hex.EncodeToString(hash[:]), StartByte: int64(start), EndByte: int64(end),
		StartLine: int64(startLine), EndLine: int64(endPhysicalLine), StartByteColumn: 0,
		EndByteColumn: int64(endColumn),
	}
}

func permitted(doc *Manifest, permit func(string) bool) error {
	if permit == nil {
		return nil
	}
	for i, source := range doc.SourceRoots {
		if source.Path != "." && !permit(source.Path) {
			return fail(fmt.Sprintf("$.source_roots[%d].path", i), "source root is outside caller scope")
		}
	}
	for i, source := range doc.DocumentSources {
		if !permit(source.Path) {
			return fail(fmt.Sprintf("$.document_sources[%d].path", i), "document source is outside caller scope")
		}
	}
	for i, source := range doc.ArtifactSources {
		if !permit(source.Path) {
			return fail(fmt.Sprintf("$.artifact_sources[%d].path", i), "artifact source is outside caller scope")
		}
	}
	for i, input := range doc.ProviderInputs {
		if !permit(input.Path) {
			return fail(fmt.Sprintf("$.provider_inputs[%d].path", i), "provider input is outside caller scope")
		}
	}
	return nil
}

func maxNodeLine(node *yaml.Node) int {
	maximum := node.Line
	for _, child := range node.Content {
		maximum = max(maximum, maxNodeLine(child))
	}
	return maximum
}

func lineStarts(data []byte) []int {
	starts := []int{0}
	for i, b := range data {
		if b == '\n' && i+1 < len(data) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func position(data []byte, offset int) (int, int) {
	line, column := 1, 0
	for _, b := range data[:offset] {
		if b == '\n' {
			line, column = line+1, 0
		} else {
			column++
		}
	}
	return line, column
}

func qualify(namespace, id string) string { return namespace + ":" + id }

func qualifyAll(namespace string, ids []string) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = qualify(namespace, id)
	}
	return result
}

func irScopes(scopes []Scope) []ir.Scope {
	result := make([]ir.Scope, len(scopes))
	for i, scope := range scopes {
		result[i] = ir.Scope{Kind: scope.Kind, Path: scope.Path}
	}
	return result
}
