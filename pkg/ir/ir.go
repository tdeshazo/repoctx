package ir

// Repository is the compact, typed representation emitted by repoctx.
// References are integer indexes; repeated strings are interned in Strings.
type Repository struct {
	Version     string       `json:"v"`
	Root        int          `json:"r,omitempty"` // string-table index
	Files       []File       `json:"f"`
	Symbols     []Symbol     `json:"s,omitempty"`
	Edges       []Edge       `json:"e,omitempty"`
	Graph       *SymbolGraph `json:"g,omitempty"`
	Diagnostics []Diagnostic `json:"d,omitempty"`
	Strings     []string     `json:"q,omitempty"`
}

type Language uint8

const (
	LangUnknown Language = iota
	LangGo
	LangPython
	LangHTML
	LangCSS
	LangJavaScript
	LangTypeScript
	LangTSX
	LangMarkdown
)

type File struct {
	Path  int      `json:"p"` // string-table index
	Lang  Language `json:"l"`
	Hash  string   `json:"h,omitempty"` // SHA-256 content hash (64 hex characters in v1alpha3)
	Unit  int      `json:"u,omitempty"` // Go package / Python module string-table index
	Nodes []Node   `json:"n,omitempty"`
	Roots []int    `json:"a,omitempty"`
}

// Node is a normalized AST node. Kind and Text are string-table indexes.
// Children contains node indexes within the same file.
type Node struct {
	Kind     int   `json:"k"`
	Text     int   `json:"t,omitempty"`
	Span     Span  `json:"x"`
	Children []int `json:"c,omitempty"`
}

type Span struct {
	SL int `json:"a"` // start line, 1-based
	SC int `json:"b"` // start UTF-8 byte column, 0-based
	EL int `json:"c"` // end line, 1-based
	EC int `json:"d"` // end UTF-8 byte column, 0-based, exclusive
}

type SymbolKind uint8

const (
	SymUnknown SymbolKind = iota
	SymModule
	SymType
	SymFunction
	SymMethod
	SymVariable
	SymConstant
)

type Symbol struct {
	ID       int        `json:"i"` // stable semantic ID string-table index
	Name     int        `json:"n"`
	Kind     SymbolKind `json:"k"`
	File     int        `json:"f"`
	Node     int        `json:"a"`
	Receiver int        `json:"r,omitempty"` // Go receiver name, before/after package linking
	Parent   int        `json:"p,omitempty"` // global symbol index + 1; zero means none
}

type EdgeKind uint8

const (
	EdgeUnknown EdgeKind = iota
	EdgeDefines
	EdgeImports
	EdgeCalls
	EdgeReferences
)

type Ref struct {
	File int `json:"f"`
	Node int `json:"n"`
}

type Edge struct {
	Kind        EdgeKind `json:"k"`
	From        Ref      `json:"f"`
	OwnerSymbol int      `json:"o,omitempty"` // owning symbol index + 1; zero means unit
	ToSymbol    int      `json:"s,omitempty"` // global symbol index + 1 if uniquely resolved
	Text        int      `json:"t,omitempty"` // unresolved target/import string
}

type Severity uint8

const (
	SeverityInfo Severity = iota + 1
	SeverityWarning
	SeverityError
)

type Diagnostic struct {
	Severity Severity `json:"s"`
	File     int      `json:"f,omitempty"` // file index + 1; zero means repository-level
	Message  int      `json:"m"`           // string-table index
}
