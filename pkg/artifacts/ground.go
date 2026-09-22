package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

const (
	// DocumentLinkProviderName identifies the Markdown link grounding provider.
	DocumentLinkProviderName = "repoctx.document-links"
	// DocumentLinkProviderVersion identifies its resolution and coverage semantics.
	DocumentLinkProviderVersion = "repoctx.document-links/v1"
	// GoImportProviderName identifies the Go import grounding provider.
	GoImportProviderName = "repoctx.go-imports"
	// GoImportProviderVersion identifies its resolution and coverage semantics.
	GoImportProviderVersion = "repoctx.go-imports/v1"
	defaultGroundingLimit   = 4096
	maximumGroundingLimit   = 16384
)

// GroundOptions selects optional, non-executing relationship adapters. A nil
// GoImports request disables the Go provider; no provider is inferred from
// repository declarations.
type GroundOptions struct {
	DocumentLinks    bool
	GoImports        *GoImportOptions
	MaxRelationships int
	MaxDiagnostics   int
}

// GoImportOptions supplies the go.mod bytes required to resolve imports within
// one module. The bytes must match the compilation manifest; the provider does
// not read files or invoke the Go tool.
type GoImportOptions struct {
	GoMod []byte
}

// Grounding contains bounded relationship evidence. It does not confer
// authority, prove runtime dispatch, or execute a provider or check.
type Grounding struct {
	Relationships []GroundedRelationship
	Diagnostics   []GroundingDiagnostic
	Providers     []ProviderMetadata
	Omissions     GroundingOmissions
	Incomplete    bool
}

// GroundedRelationship separates a claim or observed dependency by resolution
// method. ImportAlias is populated only for explicitly aliased Go imports.
type GroundedRelationship struct {
	From             string
	To               string
	Kind             string
	Resolution       string
	Provider         string
	ImportAlias      string
	TargetLabel      string
	DeclarationIndex *int
	Sites            []GroundingSite
}

// GroundingSite identifies the source span supporting a relationship. A byte
// range is present only when the artifact declaration supplied one; indexed
// syntax sites retain verified line and UTF-8 byte-column coordinates.
type GroundingSite struct {
	Path      string
	SHA256    string
	Span      GroundingSpan
	ByteRange *ByteRange
	Verified  bool
}

// GroundingSpan uses one-based lines and zero-based UTF-8 byte columns.
type GroundingSpan struct {
	StartLine       int64
	StartByteColumn int64
	EndLine         int64
	EndByteColumn   int64
}

// ByteRange is a half-open source byte range.
type ByteRange struct {
	Start int64
	End   int64
}

// GroundingDiagnostic preserves an unresolved or ambiguous target without
// manufacturing an edge. Index is set for declared catalog relationships.
type GroundingDiagnostic struct {
	Code     string
	Provider string
	From     string
	Target   string
	Path     string
	Index    *int
}

// ProviderMetadata records the implementation and exact inputs behind observed
// relationships. InputID is content-derived and is not an authenticity claim.
type ProviderMetadata struct {
	Name     string
	Version  string
	InputID  string
	Coverage ProviderCoverage
}

// ProviderCoverage states what a provider resolves and what remains outside its
// claims. Limits are stable descriptions rather than inferred capabilities.
type ProviderCoverage struct {
	Languages   []string
	Kinds       []string
	Resolutions []string
	Limits      []string
}

// GroundingOmissions counts records dropped by caller-selected output limits.
type GroundingOmissions struct {
	Relationships int
	Diagnostics   int
}

// GroundRelationships resolves declared artifact endpoints and explicitly
// requested repository-local adapters against a validated index. It performs no
// filesystem, network, build-tool, plugin, or process access.
func GroundRelationships(
	repo *ir.Repository,
	catalog *Catalog,
	options GroundOptions,
) (*Grounding, error) {
	if repo == nil {
		return nil, fmt.Errorf("grounding relationships: nil repository")
	}
	if err := repo.Validate(); err != nil {
		return nil, fmt.Errorf("grounding relationships: invalid repository: %w", err)
	}
	if catalog != nil {
		if err := validate(catalog, ceilings()); err != nil {
			return nil, fmt.Errorf("grounding relationships: invalid catalog: %w", err)
		}
	}
	maxRelationships, err := groundingLimit(options.MaxRelationships)
	if err != nil {
		return nil, fmt.Errorf("grounding relationships: max relationships: %w", err)
	}
	maxDiagnostics, err := groundingLimit(options.MaxDiagnostics)
	if err != nil {
		return nil, fmt.Errorf("grounding relationships: max diagnostics: %w", err)
	}
	modulePath := ""
	if options.GoImports != nil {
		modulePath, err = verifiedGoModule(repo, options.GoImports.GoMod)
		if err != nil {
			return nil, err
		}
	}

	g := &Grounding{
		Relationships: []GroundedRelationship{},
		Diagnostics:   []GroundingDiagnostic{},
		Providers:     []ProviderMetadata{},
	}
	collector := groundingCollector{
		grounding:        g,
		maxRelationships: maxRelationships,
		maxDiagnostics:   maxDiagnostics,
	}
	if catalog != nil {
		collector.addDeclarations(catalog)
	}
	if options.DocumentLinks {
		provider, err := documentLinkProvider(repo)
		if err != nil {
			return nil, err
		}
		g.Providers = append(g.Providers, provider)
		collector.addDocumentLinks(repo)
	}
	if options.GoImports != nil {
		provider, err := goImportProvider(repo, modulePath)
		if err != nil {
			return nil, err
		}
		g.Providers = append(g.Providers, provider)
		collector.addGoImports(repo, modulePath)
	}
	sortGrounding(g)
	g.Incomplete = g.Omissions.Relationships > 0 || g.Omissions.Diagnostics > 0
	return g, nil
}

type groundingCollector struct {
	grounding        *Grounding
	maxRelationships int
	maxDiagnostics   int
}

func (c *groundingCollector) relationship(relationship GroundedRelationship) {
	if len(c.grounding.Relationships) >= c.maxRelationships {
		c.grounding.Omissions.Relationships++
		return
	}
	c.grounding.Relationships = append(c.grounding.Relationships, relationship)
}

func (c *groundingCollector) diagnostic(diagnostic GroundingDiagnostic) {
	if len(c.grounding.Diagnostics) >= c.maxDiagnostics {
		c.grounding.Omissions.Diagnostics++
		return
	}
	c.grounding.Diagnostics = append(c.grounding.Diagnostics, diagnostic)
}

func (c *groundingCollector) addDeclarations(catalog *Catalog) {
	byID := make(map[string][]int, len(catalog.Artifacts))
	for i, artifact := range catalog.Artifacts {
		byID[artifact.ID] = append(byID[artifact.ID], i)
	}
	for i, artifact := range catalog.Artifacts {
		if len(byID[artifact.ID]) != 1 || artifact.Owner == nil {
			continue
		}
		index := i
		c.relationship(GroundedRelationship{
			From:             "artifact:" + artifact.ID,
			To:               "owner-label",
			Kind:             "owned_by",
			Resolution:       "declared_ownership",
			Provider:         "catalog",
			TargetLabel:      *artifact.Owner,
			DeclarationIndex: &index,
			Sites:            declaredSites(artifact.Sources),
		})
	}
	for i, relationship := range catalog.Relationships {
		unresolved := false
		for _, endpoint := range []string{relationship.From, relationship.To} {
			indexes := byID[endpoint]
			if len(indexes) == 1 {
				continue
			}
			code := "declared_target_unresolved"
			if len(indexes) > 1 {
				code = "declared_target_ambiguous"
			}
			index := i
			c.diagnostic(GroundingDiagnostic{
				Code: code, Provider: "catalog", Target: endpoint, Index: &index,
			})
			unresolved = true
		}
		if unresolved {
			continue
		}
		index := i
		c.relationship(GroundedRelationship{
			From:             "artifact:" + relationship.From,
			To:               "artifact:" + relationship.To,
			Kind:             relationship.Kind,
			Resolution:       "declared",
			Provider:         "catalog",
			DeclarationIndex: &index,
			Sites:            declaredSites(relationship.Sources),
		})
	}
}

func (c *groundingCollector) addDocumentLinks(repo *ir.Repository) {
	files := make(map[string]bool, len(repo.Files))
	for _, file := range repo.Files {
		files[repo.String(file.Path)] = true
	}
	for _, edge := range repo.Edges {
		if edge.Kind != ir.EdgeReferences || edge.From.File < 0 || edge.From.File >= len(repo.Files) {
			continue
		}
		file := repo.Files[edge.From.File]
		if file.Lang != ir.LangMarkdown {
			continue
		}
		from := repo.String(file.Path)
		raw := repo.String(edge.Text)
		if nodeKind(repo, edge.From) == "email_autolink" {
			continue
		}
		target, fragment, local, err := localDocumentTarget(from, raw)
		if err != nil {
			c.diagnostic(GroundingDiagnostic{
				Code: "document_target_invalid", Provider: DocumentLinkProviderName,
				From: "file:" + from, Target: raw, Path: from,
			})
			continue
		}
		if !local {
			continue
		}
		if !files[target] {
			c.diagnostic(GroundingDiagnostic{
				Code: "document_target_unavailable_in_index", Provider: DocumentLinkProviderName,
				From: "file:" + from, Target: raw, Path: from,
			})
			continue
		}
		to := "file:" + target
		if fragment != "" {
			to += "#" + fragment
		}
		c.relationship(GroundedRelationship{
			From:       "file:" + from,
			To:         to,
			Kind:       "references",
			Resolution: "syntactic",
			Provider:   DocumentLinkProviderName,
			Sites:      []GroundingSite{indexedSite(repo, edge.From)},
		})
	}
}

func (c *groundingCollector) addGoImports(repo *ir.Repository, modulePath string) {
	targets := goPackageTargets(repo, modulePath)
	for _, edge := range repo.Edges {
		if edge.Kind != ir.EdgeImports || edge.From.File < 0 || edge.From.File >= len(repo.Files) {
			continue
		}
		file := repo.Files[edge.From.File]
		if file.Lang != ir.LangGo {
			continue
		}
		raw := repo.String(edge.Text)
		if raw != modulePath && !strings.HasPrefix(raw, modulePath+"/") {
			continue
		}
		from := goUnit(repo, edge.From.File)
		resolved := targets[raw]
		if len(resolved) != 1 {
			code := "provider_target_unresolved"
			if len(resolved) > 1 {
				code = "provider_target_ambiguous"
			}
			c.diagnostic(GroundingDiagnostic{
				Code: code, Provider: GoImportProviderName,
				From: from, Target: raw, Path: repo.String(file.Path),
			})
			continue
		}
		c.relationship(GroundedRelationship{
			From:        from,
			To:          resolved[0],
			Kind:        "depends_on",
			Resolution:  "provider_resolved",
			Provider:    GoImportProviderName,
			ImportAlias: goImportAlias(repo, edge.From),
			Sites:       []GroundingSite{indexedSite(repo, edge.From)},
		})
	}
}

func declaredSites(spans []Span) []GroundingSite {
	sites := make([]GroundingSite, 0, len(spans))
	for _, source := range spans {
		sites = append(sites, GroundingSite{
			Path: source.Path, SHA256: source.SHA256,
			Span: GroundingSpan{
				StartLine: source.StartLine, StartByteColumn: source.StartByteColumn,
				EndLine: source.EndLine, EndByteColumn: source.EndByteColumn,
			},
			ByteRange: &ByteRange{Start: source.StartByte, End: source.EndByte},
		})
	}
	return sites
}

func indexedSite(repo *ir.Repository, ref ir.Ref) GroundingSite {
	file := repo.Files[ref.File]
	span := file.Nodes[ref.Node].Span
	return GroundingSite{
		Path: repo.String(file.Path), SHA256: file.Hash,
		Span: GroundingSpan{
			StartLine: int64(span.SL), StartByteColumn: int64(span.SC),
			EndLine: int64(span.EL), EndByteColumn: int64(span.EC),
		},
		Verified: true,
	}
}

func localDocumentTarget(from, raw string) (string, string, bool, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", true, err
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(raw, "//") {
		return "", "", false, nil
	}
	destination, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", "", true, err
	}
	if destination == "" {
		destination = from
	} else if strings.HasPrefix(destination, "/") {
		destination = strings.TrimPrefix(destination, "/")
	} else {
		destination = path.Join(path.Dir(from), destination)
	}
	destination = path.Clean(destination)
	if destination == "." || destination == ".." || strings.HasPrefix(destination, "../") {
		return "", "", true, fmt.Errorf("target escapes repository")
	}
	return destination, parsed.Fragment, true, nil
}

func goPackageTargets(repo *ir.Repository, modulePath string) map[string][]string {
	targets := make(map[string][]string)
	seen := make(map[string]bool)
	for i, file := range repo.Files {
		if file.Lang != ir.LangGo {
			continue
		}
		filePath := repo.String(file.Path)
		if strings.HasSuffix(filePath, "_test.go") {
			continue
		}
		dir := path.Dir(filePath)
		importPath := modulePath
		if dir != "." {
			importPath += "/" + dir
		}
		unit := goUnit(repo, i)
		key := importPath + "\x00" + unit
		if seen[key] {
			continue
		}
		seen[key] = true
		targets[importPath] = append(targets[importPath], unit)
	}
	for importPath := range targets {
		sort.Strings(targets[importPath])
	}
	return targets
}

func goUnit(repo *ir.Repository, fileIndex int) string {
	file := repo.Files[fileIndex]
	dir := path.Dir(repo.String(file.Path))
	key := repo.String(file.Unit)
	if dir != "." {
		key = dir + "/" + key
	}
	return "unit:go:" + key
}

func goImportAlias(repo *ir.Repository, ref ir.Ref) string {
	file := repo.Files[ref.File]
	if ref.Node < 0 || ref.Node >= len(file.Nodes) {
		return ""
	}
	for _, childIndex := range file.Nodes[ref.Node].Children {
		if childIndex < 0 || childIndex >= len(file.Nodes) {
			continue
		}
		child := file.Nodes[childIndex]
		if repo.String(child.Kind) == "Ident" {
			return repo.String(child.Text)
		}
	}
	return ""
}

func nodeKind(repo *ir.Repository, ref ir.Ref) string {
	if ref.File < 0 || ref.File >= len(repo.Files) {
		return ""
	}
	file := repo.Files[ref.File]
	if ref.Node < 0 || ref.Node >= len(file.Nodes) {
		return ""
	}
	return repo.String(file.Nodes[ref.Node].Kind)
}

func documentLinkProvider(repo *ir.Repository) (ProviderMetadata, error) {
	if err := providerSupports(repo, ir.LangMarkdown, "document link"); err != nil {
		return ProviderMetadata{}, err
	}
	inputID, err := providerInputID(repo, DocumentLinkProviderVersion, "")
	if err != nil {
		return ProviderMetadata{}, err
	}
	return ProviderMetadata{
		Name: DocumentLinkProviderName, Version: DocumentLinkProviderVersion, InputID: inputID,
		Coverage: ProviderCoverage{
			Languages: []string{"markdown"}, Kinds: []string{"references"},
			Resolutions: []string{"syntactic"},
			Limits: []string{
				"repository-local destinations present in the validated index only",
				"external URIs are outside coverage",
				"fragment existence is not verified",
			},
		},
	}, nil
}

func goImportProvider(repo *ir.Repository, modulePath string) (ProviderMetadata, error) {
	if err := providerSupports(repo, ir.LangGo, "go import"); err != nil {
		return ProviderMetadata{}, err
	}
	inputID, err := providerInputID(repo, GoImportProviderVersion, modulePath)
	if err != nil {
		return ProviderMetadata{}, err
	}
	return ProviderMetadata{
		Name: GoImportProviderName, Version: GoImportProviderVersion, InputID: inputID,
		Coverage: ProviderCoverage{
			Languages: []string{"go"}, Kinds: []string{"depends_on"},
			Resolutions: []string{"provider_resolved"},
			Limits: []string{
				"exact same-module import paths only",
				"external and standard-library imports are outside coverage",
				"test files are not import targets",
				"no build-tag, type, or runtime dispatch resolution",
			},
		},
	}, nil
}

func providerSupports(repo *ir.Repository, language ir.Language, label string) error {
	for _, diagnostic := range repo.Diagnostics {
		if diagnostic.Severity != ir.SeverityError || diagnostic.File == 0 {
			continue
		}
		fileIndex := diagnostic.File - 1
		if fileIndex >= 0 && fileIndex < len(repo.Files) && repo.Files[fileIndex].Lang == language {
			return fmt.Errorf(
				"grounding relationships: %s provider does not support parse-incomplete input",
				label,
			)
		}
	}
	return nil
}

func providerInputID(repo *ir.Repository, version, configuration string) (string, error) {
	snapshotID, err := repo.SnapshotID()
	if err != nil {
		return "", fmt.Errorf("grounding relationships: provider snapshot: %w", err)
	}
	return ir.ContentID(struct {
		SnapshotID    string
		Version       string
		Configuration string
	}{SnapshotID: snapshotID, Version: version, Configuration: configuration})
}

func validModulePath(modulePath string) bool {
	return modulePath != "" && path.Clean(modulePath) == modulePath &&
		!strings.HasPrefix(modulePath, ".") && !strings.HasPrefix(modulePath, "/") &&
		!strings.ContainsAny(modulePath, "\\\x00: \t\r\n")
}

func verifiedGoModule(repo *ir.Repository, data []byte) (string, error) {
	if repo.Inputs == nil || repo.Inputs.GoMod.State != "present" {
		return "", fmt.Errorf("grounding relationships: go import provider requires indexed go.mod bytes")
	}
	input := repo.Inputs.GoMod
	digest := sha256.Sum256(data)
	if int64(len(data)) != input.Bytes || hex.EncodeToString(digest[:]) != input.SHA256 {
		return "", fmt.Errorf("grounding relationships: go.mod bytes do not match compilation input")
	}
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) == 0 || strings.HasPrefix(fields[0], "//") {
			continue
		}
		if fields[0] != "module" {
			continue
		}
		if len(fields) != 2 {
			break
		}
		modulePath := fields[1]
		if strings.HasPrefix(modulePath, "\"") || strings.HasPrefix(modulePath, "`") {
			unquoted, err := strconv.Unquote(modulePath)
			if err != nil {
				break
			}
			modulePath = unquoted
		}
		if validModulePath(modulePath) {
			return modulePath, nil
		}
		break
	}
	return "", fmt.Errorf("grounding relationships: go.mod has no supported module directive")
}

func groundingLimit(limit int) (int, error) {
	if limit == 0 {
		return defaultGroundingLimit, nil
	}
	if limit < 0 || limit > maximumGroundingLimit {
		return 0, fmt.Errorf("must be between 1 and %d", maximumGroundingLimit)
	}
	return limit, nil
}

func sortGrounding(g *Grounding) {
	sort.SliceStable(g.Relationships, func(i, j int) bool {
		a, b := g.Relationships[i], g.Relationships[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Resolution < b.Resolution
	})
	sort.SliceStable(g.Diagnostics, func(i, j int) bool {
		a, b := g.Diagnostics[i], g.Diagnostics[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Path < b.Path
	})
}
