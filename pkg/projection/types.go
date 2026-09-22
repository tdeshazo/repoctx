// Package projection defines a bounded, data-only contract for lowering
// canonical context into consumer-specific forms. It does not write vendor
// files, execute commands, grant authority, or interpret repository text.
package projection

import "fmt"

// Version identifies the projection contract independently of any target.
const Version = "repoctx.projection/v1alpha1"

// Lane identifies one semantically distinct projection lane. Repository
// evidence is always data; it is never copied into the imperative lane.
type Lane string

const (
	LaneImperativeCore          Lane = "imperative_core"
	LaneTaskContract            Lane = "task_contract"
	LaneRepositoryEvidence      Lane = "repository_evidence"
	LaneExecutionAffordances    Lane = "execution_affordances"
	LaneVerificationObligations Lane = "verification_obligations"
)

// Capability names are advertised by targets. Unknown capabilities are
// rejected so a target cannot silently claim semantics this package does not
// validate.
const (
	CapabilityImperativeCore          = string(LaneImperativeCore)
	CapabilityTaskContract            = string(LaneTaskContract)
	CapabilityRepositoryEvidence      = string(LaneRepositoryEvidence)
	CapabilityExecutionAffordances    = string(LaneExecutionAffordances)
	CapabilityVerificationObligations = string(LaneVerificationObligations)
	CapabilityEscaping                = "escaping"
	CapabilitySourceMaps              = "source_maps"
)

// EscapeMode describes the target's declared representation rule. The
// contract itself is serialized as JSON, so JSON string escaping is the only
// currently implemented safe lowering mode.
type EscapeMode string

const (
	EscapeNone EscapeMode = "none"
	EscapeJSON EscapeMode = "json"
)

// Limits bound target output. Zero selects conservative defaults. Limits are
// applied after capability negotiation and every dropped record is explicit in
// Omissions.
type Limits struct {
	MaxBytes         int `json:"max_bytes"`
	MaxItems         int `json:"max_items"`
	MaxEvidenceBytes int `json:"max_evidence_bytes"`
	MaxSourceMaps    int `json:"max_source_maps"`
}

// Target describes a consumer without naming a vendor. Its capabilities are
// feature declarations, not permission grants or execution authorization.
type Target struct {
	ID           string     `json:"id"`
	Capabilities []string   `json:"capabilities"`
	Limits       Limits     `json:"limits"`
	Escape       EscapeMode `json:"escape"`
}

// Contract is the caller-approved, model-neutral input to negotiation. The
// lane fields contain declarations and evidence as data; this package does
// not infer policy or promote prose into imperative instructions.
type Contract struct {
	Version      string             `json:"version"`
	ID           string             `json:"id"`
	Core         []CoreItem         `json:"imperative_core"`
	Task         []TaskItem         `json:"task_contract"`
	Evidence     []EvidenceItem     `json:"repository_evidence"`
	Execution    []ExecutionItem    `json:"execution_affordances"`
	Verification []VerificationItem `json:"verification_obligations"`
	SourceMaps   []SourceMap        `json:"source_maps"`
}

// CoreItem is a caller-supplied imperative declaration. Text is opaque to
// this package and is never derived from repository evidence.
type CoreItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// TaskItem describes a task contract as data, not as a command to execute.
type TaskItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// EvidenceItem is untrusted repository evidence. Its text remains in this
// lane and is JSON-escaped by the deterministic encoder.
type EvidenceItem struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"`
	Text   string    `json:"text"`
	Source SourceRef `json:"source"`
}

// ExecutionItem describes an affordance without activating it. No command,
// permission, credential, or process is executed by this package.
type ExecutionItem struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// VerificationItem describes a verification obligation without claiming a
// result or passing authority to a runner.
type VerificationItem struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
	Expected string `json:"expected"`
}

// SourceRef identifies exact source evidence without authenticating it.
type SourceRef struct {
	ID        string `json:"id,omitempty"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256,omitempty"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// SourceMap maps one projected lane item to exact source evidence. A target
// that lacks source-map support receives an explicit omission instead.
type SourceMap struct {
	Lane   Lane      `json:"lane"`
	ItemID string    `json:"item_id"`
	Source SourceRef `json:"source"`
}

// Omission records unsupported or budget-limited data. Count is always
// positive; omission records are sorted deterministically in the result.
type Omission struct {
	Lane    Lane   `json:"lane"`
	Feature string `json:"feature"`
	Count   int    `json:"count"`
	Reason  string `json:"reason"`
}

// Capabilities makes both supported and unsupported target features visible.
type Capabilities struct {
	Available   []string `json:"available"`
	Unavailable []string `json:"unavailable"`
}

// Projection is the negotiated data-only result. It contains no generated
// files and no execution result.
type Projection struct {
	Version      string             `json:"version"`
	Target       string             `json:"target"`
	Escape       EscapeMode         `json:"escape"`
	Capabilities Capabilities       `json:"capabilities"`
	Core         []CoreItem         `json:"imperative_core"`
	Task         []TaskItem         `json:"task_contract"`
	Evidence     []EvidenceItem     `json:"repository_evidence"`
	Execution    []ExecutionItem    `json:"execution_affordances"`
	Verification []VerificationItem `json:"verification_obligations"`
	SourceMaps   []SourceMap        `json:"source_maps"`
	Omissions    []Omission         `json:"omissions"`
	Incomplete   bool               `json:"incomplete"`
}

// Result contains the structured negotiated projection and its canonical
// newline-terminated JSON payload.
type Result struct {
	Projection Projection
	Payload    []byte
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("projection: "+format, args...)
}
