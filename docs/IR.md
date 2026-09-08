# Repository IR v1alpha3

This wire format is a reusable machine index. Agents should receive the separate
[context bundle](AGENT_CONTEXT.md), not the interned AST/CSR tables.

## Coordinates and references

| Reference | Encoding |
|---|---|
| String table `q` | Slice is zero-based; references are **index + 1**. Zero is absent. |
| File, AST node, symbol and graph node | Zero-based index. |
| Symbol parent `p`, edge target `s`, edge owner `o`, diagnostic file `f` | **Index + 1**. Zero is absent/unit-level as appropriate. |
| Source lines | One-based physical lines. Synthetic AST nodes may have line zero. |
| Source columns | Zero-based **UTF-8 byte** offsets, not Unicode characters. |
| Source end | Exclusive. |

Go positions ignore `//line` remapping. Context materialization rejects synthetic,
empty, invalid or non-UTF-8-aligned spans. Python spans include decorators for
function/class definitions. Ordinary Python comments are not AST nodes.

## Top-level record

```go
type Repository struct {
    Version     string       // v: repoctx.ir/v1alpha3
    Root        int          // r: interned ".", not an absolute host path
    Files       []File       // f
    Symbols     []Symbol     // s
    Edges       []Edge       // e: source occurrences
    Graph       *SymbolGraph // g: collapsed traversal view
    Diagnostics []Diagnostic // d
    Strings     []string     // q
}
```

Each file carries path, language (`1=Go`, `2=Python`, `3=HTML`, `4=CSS`,
`5=JavaScript`, `6=TypeScript`, `7=TSX/JSX`), a required full SHA-256 content
hash, optional unit identity, normalized node table and roots. Go uses the
native Go parser; the other six languages use pinned Tree-sitter grammars.
File paths are repository-relative slash paths. Traversal, drive/colon forms,
backslashes and duplicate paths are rejected by `Repository.Validate`.

```go
type Node struct {
    Kind     int   // k: interned front-end AST kind
    Text     int   // t: optional interned identifier/literal text
    Span     Span  // x: physical source coordinates
    Children []int // c: indexes into this file's node table
}
```

Children have greater indexes than their parent in the normalized preorder
representation. The validator rejects backward/cyclic child references. Kinds
remain language-specific (`FuncDecl`, `CallExpr`, `FunctionDef`, `Call`, etc.);
this is a shared storage shape, not a claim that both languages share semantics.

## Symbols

```go
type Symbol struct {
    ID       int        // i: interned semantic ID
    Name     int        // n
    Kind     SymbolKind // k: module/type/function/method/variable/constant
    File     int        // f
    Node     int        // a: declaration AST node
    Receiver int        // r: optional Go receiver name
    Parent   int        // p: lexical/receiver owner index + 1
}
```

Example IDs:

```text
go:pkg/compiler/compiler#Compile
go:pkg/service/service#Client.Do
py:pkg.worker#Worker.files
```

Normal IDs derive from language, repository-relative unit and lexical ownership.
Duplicate bases receive path/position/ordinal suffixes. IDs can change after
renames, scope changes, or duplicate-definition disambiguation. Store an ID with
its snapshot digest, never as an unconditional cross-revision identity.

Go variable/constant symbols refer to value declarations, not just identifier
tokens. Cross-file and simple generic method receivers link to package types.
Python local assignments and nested functions/classes retain lexical parents.
JavaScript and TypeScript declarations, HTML tags, and CSS selectors/properties
are represented as conservative syntax-level symbols when the grammar exposes
their names. TSX/JSX is syntax support only; it does not claim React runtime or
component semantics. This remains syntax-derived symbol extraction, not full
scope/type/module/browser resolution.

## Occurrence relationships

```go
type Edge struct {
    Kind        EdgeKind // k
    From        Ref      // f: source file and AST node
    OwnerSymbol int      // o: owning symbol index + 1; zero means unit
    ToSymbol    int      // s: candidate symbol index + 1; zero unresolved
    Text        int      // t: original import/callee spelling
}
```

`DEFINES=1`, `IMPORTS=2`, `CALLS=3`, `REFERENCES=4`. The current front ends produce
imports and calls; defines are built from ownership. References is reserved and
not populated. Call candidates are restricted by language and unit, then matched
by terminal name. They are **heuristic candidates**, not proof of static or runtime
dispatch. Dynamic dispatch, aliases, shadowing and imported targets can be missed
or incorrectly suggested. The agent bundle marks this limitation per relation.

## Dense graph

```go
type SymbolGraph struct {
    External []GraphExternal // x: unit/module/unresolved anchors
    Out      CSR             // o
    In       CSR             // i
}

type CSR struct {
    Offsets []uint32   // o, length nodeCount+1
    Targets []uint32   // t
    Kinds   []EdgeKind // k, base64-packed uint8 vector on the JSON wire
    Weights []uint32   // w, number of collapsed occurrences
}
```

Repository symbols occupy graph IDs `[0,len(Symbols))` directly. Other nodes have
IDs `len(Symbols)+externalIndex`. For node `n`, adjacency is the half-open slice
`Offsets[n]:Offsets[n+1]`. Forward and reverse CSR are both materialized.

Repeated `(from,to,kind)` arcs are collapsed, retaining occurrence weights. The
occurrence table keeps source locations. Unit nodes own top-level definitions and
imports. External module and unresolved-symbol nodes preserve boundary labels.
A globally merged unresolved label is not proof that its callers share a callee;
the context planner deliberately does not traverse such hubs.

Go imports use the root go.mod module name when available. Python uses module
names and longest-prefix matching. Neither provides complete import/type linking
across nested modules, relative imports, build tags, conditional code or runtime
loading. Go build targets and tests are not analyzed semantically.

## Validation, source identity and I/O

`Repository.Validate` checks versions, string/file/node/symbol indexes, hashes,
path syntax, parent cycles, AST child links, CSR offsets/parallel arrays and exact
forward/reverse consistency. `compiler.Read` rejects extra JSON documents, unknown
fields and decoded input over 128 MiB. These checks are structural, not a security
attestation or proof of language semantics.

`SnapshotID` hashes normalized index JSON. The digest binds the complete index,
including file hashes, graph, ASTs and diagnostics; it is not a Git revision or
signature. The context compiler rereads every permitted indexed file and compares
its hash before serving evidence. It does not detect additions or unindexed
configuration changes. Compile and serve from an immutable worktree, and rebuild
after repository/build-input changes.

The JSON schema is `ir.schema.json`. Gzip is appropriate for storing/transferring
indexes; compressed byte count is unrelated to model context consumption.

## Compatibility

Version 3 upgrades 24-hex truncated hashes to 64-hex SHA-256 and adds optional
symbol receiver `r` and occurrence owner `o`. It also corrects source spans and
lexical ownership. `compiler.Read` accepts version 2 for `stats` and `graph`, but
agent serving requires version 3. Recompile older indexes; merely changing their
version string does not establish the new invariants.

The public packages are `pkg/ir`, `pkg/compiler` and `pkg/agentctx`.
