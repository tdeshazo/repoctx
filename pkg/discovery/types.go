// Package discovery provides bounded, index-free repository navigation.
// Returned content is untrusted evidence, not instructions or a snapshot proof.
package discovery

import "fmt"

// Version identifies the discovery wire contract, independently of index/context.
const Version = "repoctx.discovery/v1alpha1"

// ReadRequest selects a repository-relative file and optional inclusive lines.
// Zero StartLine/EndLine means the beginning/end of the file.
type ReadRequest struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

// Options controls live discovery. Zero limits receive documented defaults.
// Scope prefixes are not globs; deny always wins over visibility overrides.
type Options struct {
	Root           string        `json:"root"`
	Operation      string        `json:"operation"`
	Query          string        `json:"query,omitempty"`
	Glob           string        `json:"glob,omitempty"`
	Type           string        `json:"type"`
	Regex          bool          `json:"regex"`
	IgnoreCase     bool          `json:"ignore_case"`
	Hidden         bool          `json:"hidden"`
	NoIgnore       bool          `json:"no_ignore"`
	AllowPaths     []string      `json:"allow"`
	DenyPaths      []string      `json:"deny"`
	Reads          []ReadRequest `json:"reads"`
	Depth          int           `json:"depth"`
	ContextLines   int           `json:"context_lines"`
	MaxBytes       int           `json:"max_bytes"`
	MaxSourceBytes int64         `json:"max_source_bytes"`
	MaxReadBytes   int64         `json:"max_read_bytes"`
	MaxResults     int           `json:"max_results"`
	MaxEntries     int           `json:"max_entries"`
	Format         string        `json:"format"`
}

// Usage counts work performed, including ignore-file reads.
type Usage struct {
	Entries     int   `json:"entries"`
	Files       int   `json:"files"`
	Directories int   `json:"directories"`
	ReadBytes   int64 `json:"read_bytes"`
}

// Omission records an unavailable result or bounded work. Empty Path is global.
type Omission struct {
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason"`
}

// Entry is a discovered path; candidate labels are filename heuristics only.
type Entry struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	Candidate string `json:"candidate,omitempty"`
}

// Score contains lexical ranking components, not confidence estimates.
type Score struct {
	DistinctTerms int `json:"distinct_terms"`
	PathTerms     int `json:"path_terms"`
	Occurrences   int `json:"occurrences"`
}

// Evidence preserves exact UTF-8 bytes and the hash of the buffer they came from.
// Byte offsets are half-open; line numbers are one-based and inclusive.
type Evidence struct {
	SHA256    string `json:"sha256"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Text      string `json:"text"`
}

// Result carries an inventory entry with optional matched terms and evidence.
type Result struct {
	Entry
	MatchedTerms []string  `json:"matched_terms"`
	Score        Score     `json:"score"`
	Evidence     *Evidence `json:"evidence,omitempty"`
	metadataRank int       `json:"-"`
}

// Response reports observed source, never an atomic filesystem snapshot.
type Response struct {
	Version    string     `json:"version"`
	Options    Options    `json:"options"`
	Usage      Usage      `json:"usage"`
	Overview   []Entry    `json:"overview"`
	Results    []Result   `json:"results"`
	Omissions  []Omission `json:"omissions"`
	Incomplete bool       `json:"incomplete"`
	Warnings   []string   `json:"warnings"`
}

// ResultSet contains the response and its final bounded serialization.
type ResultSet struct {
	Response Response
	Payload  []byte
}

// UsageError distinguishes invalid caller input from execution failures.
type UsageError struct{ Message string }

func (e *UsageError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &UsageError{Message: fmt.Sprintf(format, args...)}
}
