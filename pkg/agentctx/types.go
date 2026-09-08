// Package agentctx lowers a repository index into bounded, readable evidence
// for an agent. The index is an internal planning representation; a Bundle is a
// self-contained tool-result payload. Source text is always untrusted data.
package agentctx

import "github.com/tdeshazo/repoctx/pkg/ir"

const Version = "repoctx.context/v1alpha1"

// TokenCounter must count the exact supplied rendered bytes using the target
// model's tokenizer. No generic chars/token estimate is used as a token bound.
type TokenCounter func([]byte) (int, error)

type Options struct {
	Root             string
	Query            string
	Symbols          []string      // exact semantic IDs, never persistent dense IDs
	ExpectedSnapshot string        // optional sha256: index digest, for expansion/replay
	Depth            int           // 0 = seeds only; at most 4
	Direction        string        // out, in, both (default)
	Relations        []ir.EdgeKind // default: calls, defines; imports rendered as evidence
	MaxSymbols       int           // default 12; at most 64
	MaxCandidates    int           // default 128; at most 4096
	MaxRelations     int           // default 48
	MaxBytes         int           // exact serialized payload cap; default 32768
	Format           string        // json (default) or markdown
	AllowPaths       []string      // repository-relative file/directory prefixes, not globs
	DenyPaths        []string      // deny wins; applied before ranking, source reads and serving
	MaxSourceBytes   int64         // per-file bound; default 2 MiB
	MaxReadBytes     int64         // total verified source bytes; default 256 MiB
	MaxTokens        int           // optional exact payload token cap; requires CountTokens
	CountTokens      TokenCounter
}

type Bundle struct {
	Version       string         `json:"version"`
	Snapshot      Snapshot       `json:"snapshot"`
	Query         string         `json:"query,omitempty"`
	Seeds         []string       `json:"seeds"`
	Trust         Trust          `json:"trust"`
	Capabilities  Capabilities   `json:"capabilities"`
	Selection     Selection      `json:"selection"`
	Symbols       []Symbol       `json:"symbols"`
	Evidence      []Evidence     `json:"evidence"`
	Relationships []Relationship `json:"relationships,omitempty"`
	Omissions     Omissions      `json:"omissions"`
	Warnings      []string       `json:"warnings"`
}

type Snapshot struct {
	ID            string `json:"id"`
	IRVersion     string `json:"ir_version"`
	Verification  string `json:"verification"`
	VerifiedFiles int    `json:"verified_files"`
}
type Trust struct {
	Role     string `json:"role"`
	Handling string `json:"handling"`
}
type Capabilities struct {
	Available      []string `json:"available"`
	Unavailable    []string `json:"unavailable"`
	CallResolution string   `json:"call_resolution"`
	Verification   string   `json:"verification"`
}
type Selection struct {
	Strategy      string   `json:"strategy"`
	Depth         int      `json:"depth"`
	Direction     string   `json:"direction"`
	Relations     []string `json:"relations"`
	MaxBytes      int      `json:"max_bytes"`
	MaxTokens     int      `json:"max_tokens,omitempty"`
	MaxSymbols    int      `json:"max_symbols"`
	MaxCandidates int      `json:"max_candidates"`
	MaxRelations  int      `json:"max_relations"`
}
type Span struct {
	StartLine       int `json:"start_line"`
	StartByteColumn int `json:"start_byte_column"`
	EndLine         int `json:"end_line"`
	EndByteColumn   int `json:"end_byte_column"`
}

func span(x ir.Span) Span { return Span{x.SL, x.SC, x.EL, x.EC} }

type Reason struct {
	Strategy  string `json:"strategy"`
	Depth     int    `json:"depth"`
	Score     int    `json:"score"` // ranking signal, not probability or confidence
	Via       string `json:"via,omitempty"`
	Relation  string `json:"relation,omitempty"`
	Direction string `json:"direction,omitempty"`
}
type Symbol struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Language     string `json:"language"`
	File         string `json:"file"`
	Unit         string `json:"unit,omitempty"`
	Definition   Span   `json:"definition"`
	Evidence     string `json:"evidence"`     // evidence ID; may be shared with another symbol
	Completeness string `json:"completeness"` // full_definition or declaration_excerpt
	Reason       Reason `json:"reason"`
}
type Evidence struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	SHA256    string `json:"sha256"`
	Span      Span   `json:"span"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	Role      string `json:"role"` // source or imports; always untrusted repository data
	Text      string `json:"text"`
}
type Location struct {
	File   string `json:"file"`
	Span   Span   `json:"span"`
	SHA256 string `json:"sha256"`
}
type Relationship struct {
	From        string     `json:"from"`
	To          string     `json:"to"`
	Kind        string     `json:"kind"`
	Resolution  string     `json:"resolution"` // syntactic, name_heuristic, unresolved
	Occurrences uint32     `json:"occurrences"`
	Sites       []Location `json:"sites,omitempty"` // up to three source occurrences
}
type Omissions struct {
	Candidates       int  `json:"candidates"`
	Budget           int  `json:"budget"`
	SymbolLimit      int  `json:"symbol_limit"`
	Excerpts         int  `json:"excerpts"`
	Relations        int  `json:"relations"`
	Imports          int  `json:"imports"`
	TraversalLimited bool `json:"traversal_limited"`
}
type Usage struct {
	Bytes       int    `json:"bytes"`
	Tokens      int    `json:"tokens"`
	TokenMethod string `json:"token_method"`
	ExactTokens bool   `json:"exact_tokens"`
}
type Result struct {
	Bundle  *Bundle
	Payload []byte // includes final newline; write these bytes without re-encoding
	Usage   Usage
}
