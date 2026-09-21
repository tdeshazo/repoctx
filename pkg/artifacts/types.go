// Package artifacts decodes untrusted, opt-in artifact declarations and exposes
// explicit, non-executing authority and relationship stages. Structural
// acceptance does not verify source bytes, grant authority, discover catalogs,
// read declared paths, activate providers, or execute runner check IDs.
package artifacts

// Version identifies the standalone artifact wire contract.
const Version = "repoctx.artifacts/v1alpha1"

// Catalog retains declaration order, duplicates, and conflicting claims.
type Catalog struct {
	Version       string         `json:"version"`
	Namespace     string         `json:"namespace"`
	Artifacts     []Artifact     `json:"artifacts"`
	Relationships []Relationship `json:"relationships"`
}

// Artifact is an author-assigned identity and its untrusted metadata claims.
type Artifact struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Owner          *string         `json:"owner,omitempty"`
	AppliesTo      []Applicability `json:"applies_to"`
	Lifecycle      string          `json:"lifecycle"`
	Sources        []Span          `json:"sources"`
	DeclaredInputs []Input         `json:"declared_inputs"`
	RunnerCheckID  *string         `json:"runner_check_id,omitempty"`
}

// Applicability declares a file or subtree extent without granting access.
type Applicability struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Span contains unverified claims about original bytes. Endpoints are exclusive;
// columns count UTF-8 bytes, lines split on LF, and CR is retained.
type Span struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	StartByte       int64  `json:"start_byte"`
	EndByte         int64  `json:"end_byte"`
	StartLine       int64  `json:"start_line"`
	EndLine         int64  `json:"end_line"`
	StartByteColumn int64  `json:"start_byte_column"`
	EndByteColumn   int64  `json:"end_byte_column"`
}

// Input declares a dependency without asserting availability or granting reads.
type Input struct {
	Path     string  `json:"path"`
	Role     string  `json:"role"`
	Required bool    `json:"required"`
	SHA256   *string `json:"sha256,omitempty"`
}

// Relationship preserves an unresolved directed claim and its provenance.
type Relationship struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Resolution string `json:"resolution"`
	Sources    []Span `json:"sources"`
}

// Limits can lower the production ceilings. Zero selects the ceiling; negative
// values or values above a ceiling are errors. Limits are inclusive.
type Limits struct {
	Bytes         int
	Depth         int
	Artifacts     int
	Relationships int
	Applicability int
	Inputs        int
	Sources       int
	NestedEntries int
}

// Authority is trusted caller configuration for resolving repository claims.
// Artifact declarations cannot add accepted IDs or widen these scopes.
type Authority struct {
	AcceptedIDs []string
	Scopes      []Applicability
}

// EffectiveArtifact is an unambiguous declaration accepted by the caller for
// the listed intersections of declared and caller-controlled scope.
type EffectiveArtifact struct {
	ID               string
	Kind             string
	LifecycleClaim   string
	DeclarationIndex int
	Scopes           []Applicability
}

// Diagnostic describes a declaration problem without copying repository text.
// Index fields identify the original ordered declarations when applicable.
type Diagnostic struct {
	Code                string
	ArtifactIDs         []string
	ArtifactIndexes     []int
	RelationshipIndexes []int
	Path                string
}

// Resolution contains caller-authorized artifacts and deterministic diagnostics.
// Diagnostics never grant authority and relationships remain declarations.
type Resolution struct {
	Artifacts   []EffectiveArtifact
	Diagnostics []Diagnostic
}

func ceilings() Limits {
	return Limits{Bytes: 1 << 20, Depth: 16, Artifacts: 1024, Relationships: 4096,
		Applicability: 64, Inputs: 64, Sources: 16, NestedEntries: 16384}
}
