// Package diagnostic defines bounded compiler findings and deterministic JSON
// and SARIF projections. Records are untrusted data, not instructions, authority,
// coverage evidence, or proof of execution. Callers must filter denied content
// before constructing records. This package performs no I/O and executes no
// verification checks.
package diagnostic

// Version identifies the diagnostic wire contract.
const Version = "repoctx.diagnostics/v1alpha1"

// Code identifies a stable diagnostic category independently of message text.
// Unknown codes are rejected; producers must not derive codes from source text.
type Code string

const (
	CodeSyntaxError         Code = "repoctx.syntax_error"
	CodeInvalidInput        Code = "repoctx.invalid_input"
	CodeInputUnavailable    Code = "repoctx.input_unavailable"
	CodeUnresolvedReference Code = "repoctx.unresolved_reference"
	CodeStaleInput          Code = "repoctx.stale_input"
	CodePolicyDenied        Code = "repoctx.policy_denied"
	CodeProviderUnavailable Code = "repoctx.provider_unavailable"
	CodeLimitExceeded       Code = "repoctx.limit_exceeded"
	CodeInternalError       Code = "repoctx.internal_error"
)

// Severity is the producer's classification, not a check outcome.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Remediation describes the category of human follow-up, never an executable fix.
type Remediation string

const (
	RemediationSource        Remediation = "fix_source"
	RemediationConfiguration Remediation = "fix_configuration"
	RemediationRefresh       Remediation = "refresh_inputs"
	RemediationPolicy        Remediation = "review_policy"
	RemediationCapability    Remediation = "provide_capability"
	RemediationScope         Remediation = "reduce_scope"
	RemediationBug           Remediation = "report_bug"
)

// Record attributes a finding to a compiler pass and optional provider, naming
// one affected capability. Message is plain UTF-8 text and may be hostile.
// Pass, Provider and Capability are bounded identifiers, not paths or commands.
type Record struct {
	Code        Code        `json:"code"`
	Severity    Severity    `json:"severity"`
	Message     string      `json:"message"`
	Pass        string      `json:"pass"`
	Provider    string      `json:"provider,omitempty"`
	Capability  string      `json:"capability"`
	Remediation Remediation `json:"remediation"`
	Location    *Location   `json:"location,omitempty"`
}

// Location identifies an exact range in the complete source file named by Path
// and SHA256. Byte offsets are half-open; lines are one-based and columns count
// zero-based UTF-8 bytes. A zero-width range is an insertion point. Omit Location
// when exact coordinates or a source digest are unavailable; never guess them.
// Validation is structural and cannot authenticate the claimed source bytes.
type Location struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	StartByte       int64  `json:"start_byte"`
	EndByte         int64  `json:"end_byte"`
	StartLine       int64  `json:"start_line"`
	EndLine         int64  `json:"end_line"`
	StartByteColumn int64  `json:"start_byte_column"`
	EndByteColumn   int64  `json:"end_byte_column"`
}

// Report is a collection of findings. An empty collection does not establish
// coverage, a successful compilation, or a passing verification check.
type Report struct {
	Version     string   `json:"version"`
	Diagnostics []Record `json:"diagnostics"`
}

// Limits bounds accepted records and complete newline-terminated output. Zero
// uses defaults: 1024 records, 4096 message bytes, 4096 path bytes, and 1 MiB of
// output. Ceilings are 4096 records, 8192 message bytes, 4096 path bytes, and
// 8 MiB of output. Invalid limits and overflow fail without partial output.
type Limits struct {
	MaxRecords      int
	MaxMessageBytes int
	MaxPathBytes    int
	MaxBytes        int
}
