// Package manifest validates the bounded, repository-authored agent-context.yaml
// contract. A valid manifest describes inputs and capabilities; it grants no
// filesystem authority, activates no provider, and authenticates no claim.
package manifest

// Version identifies the only supported repository manifest contract.
const Version = "repoctx.manifest/v1alpha3"

// FrontmatterVersion identifies typed declarations embedded in Markdown.
const FrontmatterVersion = "repoctx.frontmatter/v1alpha1"

// Manifest is the authored semantic repository declaration. It deliberately
// contains no generated hashes, byte coordinates, observed state, or authority.
type Manifest struct {
	Version         string           `json:"version" yaml:"version"`
	Namespace       string           `json:"namespace" yaml:"namespace"`
	SourceRoots     []SourceRoot     `json:"source_roots" yaml:"source_roots"`
	DocumentSources []DocumentSource `json:"document_sources" yaml:"document_sources"`
	ArtifactSources []ArtifactSource `json:"artifact_sources" yaml:"artifact_sources"`
	Components      []Component      `json:"components" yaml:"components"`
	ProviderInputs  []ProviderInput  `json:"provider_inputs" yaml:"provider_inputs"`
	DerivedViews    []DerivedView    `json:"derived_views" yaml:"derived_views"`
	Capabilities    []string         `json:"capabilities" yaml:"capabilities"`
}

// DocumentSource identifies a Markdown file containing repoctx frontmatter.
type DocumentSource struct {
	ID     string `json:"id" yaml:"id"`
	Path   string `json:"path" yaml:"path"`
	Format string `json:"format" yaml:"format"`
}

// SourceRoot names a caller-confined repository subtree.
type SourceRoot struct {
	ID   string `json:"id" yaml:"id"`
	Path string `json:"path" yaml:"path"`
}

// ArtifactSource identifies an authored semantic declaration file and format.
type ArtifactSource struct {
	ID     string `json:"id" yaml:"id"`
	Path   string `json:"path" yaml:"path"`
	Format string `json:"format" yaml:"format"`
}

// Component associates a stable authored identity with declared inputs.
type Component struct {
	ID              string    `json:"id" yaml:"id"`
	SourceRoots     []string  `json:"source_roots" yaml:"source_roots"`
	ArtifactSources []string  `json:"artifact_sources" yaml:"artifact_sources"`
	ProviderInputs  []string  `json:"provider_inputs" yaml:"provider_inputs"`
	Owners          []string  `json:"owners" yaml:"owners"`
	Scopes          []Scope   `json:"scopes" yaml:"scopes"`
	Lifecycle       string    `json:"lifecycle" yaml:"lifecycle"`
	Supersedes      []string  `json:"supersedes" yaml:"supersedes"`
	Sensitivity     string    `json:"sensitivity" yaml:"sensitivity"`
	Freshness       Freshness `json:"freshness" yaml:"freshness"`

	// DependsOn optionally declares other manifest components as dependencies.
	// These are authored claims, not resolved imports or execution ordering.
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// Scope is an authored applicability claim.
type Scope struct {
	Kind string `json:"kind" yaml:"kind"`
	Path string `json:"path" yaml:"path"`
}

// Freshness names inputs whose changes can make a declaration stale.
type Freshness struct {
	Inputs []string `json:"inputs" yaml:"inputs"`
}

// ProviderInput identifies bytes offered to a named, non-authorizing provider.
type ProviderInput struct {
	ID       string `json:"id" yaml:"id"`
	Provider string `json:"provider" yaml:"provider"`
	Path     string `json:"path" yaml:"path"`
}

// DerivedView declares a supported deterministic view over named components.
type DerivedView struct {
	ID         string   `json:"id" yaml:"id"`
	Kind       string   `json:"kind" yaml:"kind"`
	Components []string `json:"components" yaml:"components"`
}

// Limits may lower, but never raise, the production decoding ceilings. Zero
// selects the corresponding production ceiling.
type Limits struct {
	Bytes           int
	Depth           int
	Entries         int
	SourceRoots     int
	DocumentSources int
	ArtifactSources int
	Components      int
	ProviderInputs  int
	DerivedViews    int
	Capabilities    int
}

func ceilings() Limits {
	return Limits{Bytes: 256 << 10, Depth: 12, Entries: 4096, SourceRoots: 64,
		DocumentSources: 64, ArtifactSources: 64, Components: 256, ProviderInputs: 256,
		DerivedViews: 64, Capabilities: 64}
}
