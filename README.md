# repoctx — repository index and agent context compiler

`repoctx` has **two outputs with different consumers**:

1. **Repository IR (`repoctx.ir/v1alpha3`)**: compact Go/Python/HTML/CSS/JavaScript/TypeScript/TSX/Markdown ASTs, interned
   strings, symbols, occurrence edges, and dense forward/reverse CSR adjacency.
   This is the reusable machine index, not the prompt.
2. **Agent context (`repoctx.context/v1alpha1`)**: a selected, readable JSON or
   Markdown bundle with exact source, semantic IDs, provenance, selection
   reasons, bounded relationships, explicit omissions, and trust metadata.
   Pass this as **tool-result evidence**, not as system/developer instructions.

## Build and test

```sh
go test ./...
go vet ./...
go build -o repoctx .
go build -o repoctx-cmd ./cmd/repoctx
```

Install the module-root command from its published module path:

```sh
go install github.com/tdeshazo/repoctx@latest
```

This remote install command is usable only after the repository has been
published at `github.com/tdeshazo/repoctx` and a reachable version is available
(for example, a pushed release tag or the default branch). A local checkout
cannot satisfy the remote lookup, and this prerequisite is not verified here.
The legacy `./cmd/repoctx` command remains buildable and can also be installed
from a published version with `go install github.com/tdeshazo/repoctx/cmd/repoctx@latest`.

The module retains a Go 1.23 language baseline and uses pinned official
Tree-sitter Go bindings and grammars (Python v0.25.0, HTML v0.23.2, CSS v0.25.0,
JavaScript v0.25.0, TypeScript/TSX v0.23.2, Markdown v0.4.1). A C compiler is required by the native bindings; no Python
interpreter is started or required to index Python files. The supplied executable
targets Linux amd64. Use an appropriate maintained toolchain for deployment.

`.ts`, `.mts`, and `.cts` files use the TypeScript grammar; `.tsx` and `.jsx`
files use the TSX grammar. `.js`, `.mjs`, and `.cjs` remain JavaScript and are
parsed by the separate JavaScript grammar.
`.md` files use the maintained Markdown block and inline grammars. Headings and
conservative link relationships are source-linked. Fenced-code bodies remain
raw Markdown context only and are never recursively parsed or treated as
embedded Go, TypeScript, or another language.

## Agent skill

The vendored [repoctx skill](skills/repoctx/SKILL.md) teaches agents to compile
an index, retrieve bounded evidence, check freshness and limitations, and expand
context only when needed. It assumes the `repoctx` binary is on `PATH`.

Copy the `skills/repoctx` directory into your agent's skill directory to use it
outside this checkout. Installing the Go binary does not install the skill.

## Compile, then retrieve for an agent

```sh
./repoctx compile -root examples/mixed -o mixed.ir.json.gz
./repoctx validate mixed.ir.json.gz

./repoctx context \
  -root examples/mixed \
  -query 'Worker.files' \
  -depth 1 \
  -max-bytes 20000 \
  -o context.json \
  mixed.ir.json.gz
```

JSON is the default. Status and approximate token usage go to stderr, so stdout
contains only the payload. All flags must precede the final index argument.
`-max-bytes` limits the **whole rendered, uncompressed output**, including JSON
escaping, metadata and the trailing newline. It is not a model token limit.

```sh
./repoctx context -root examples/mixed \
  -symbol 'py:pkg.worker#Worker.files' \
  -format markdown -max-bytes 20000 mixed.ir.json.gz
```

Do not send the raw CSR/string-table index to the model. Do not infer a token
count from gzip size. The CLI's bytes/4 token estimate is explicitly approximate.

## Incremental disclosure and scope

Take `snapshot.id` and a semantic ID from a returned symbol or relationship:

```sh
./repoctx context -root examples/mixed \
  -symbol 'py:pkg.worker#Worker.files' \
  -expect-snapshot 'sha256:THE_DIGEST_FROM_THE_PREVIOUS_BUNDLE' \
  -direction out -depth 2 -max-bytes 12000 mixed.ir.json.gz
```

A digest mismatch or changed indexed source fails instead of returning stale
snippets. Dense graph integers are snapshot-local and must not be durable agent
memory. Semantic IDs also need re-resolution after renames or collisions.

Caller-controlled file/directory prefixes can exclude content **before indexing**
and again **before context selection and source reads**:

```sh
./repoctx compile -root /repo -deny secrets -deny generated -o repo.ir.json.gz
./repoctx context -root /repo -query 'retry' \
  -allow services/payments -deny services/payments/private repo.ir.json.gz
```

These flags take prefixes, not glob patterns. Deny wins. Filtering context does
not scrub a previously created index: indexes contain source-derived literals
and names and must be stored as sensitive artifacts. There is no secret scanner
or automatic credential redaction.

## What agents receive

The bundle contains named types rather than opaque string-table offsets:

- **Snapshot and capabilities**: index digest, source verification scope,
  available AST/graph features, and explicit missing capabilities.
- **Symbols**: semantic ID, kind, language, file, source span, evidence reference,
  full-definition/excerpt status, and why the symbol was selected.
- **Evidence**: exact source bytes, file SHA-256, physical UTF-8 byte coordinates,
  and byte ranges. Overlapping class/member or declaration spans are merged.
- **Relationships**: named endpoints, direction, occurrence count, up to three
  source locations, and `syntactic`, `name_heuristic`, or `unresolved` status.
- **Omissions and warnings**: budget exclusions, symbol limits, excerpts, dropped
  imports/relationships, traversal limits, diagnostics and freshness limits.

The planner uses deterministic lexical seeds and bounded graph expansion. It
reserves up to a quarter of the byte budget (capped at 4096 bytes) for supporting
imports and graph evidence. It does not expand through package hubs or globally
merged unresolved names. Oversized definitions become **explicitly marked exact
source excerpts**, not invented summaries. A required seed that cannot fit causes
an error. The planner is greedy and bounded, not globally optimal.

## Public Go API

```go
import (
    "github.com/tdeshazo/repoctx/pkg/agentctx"
    "github.com/tdeshazo/repoctx/pkg/compiler"
)

index, err := compiler.Read("repo.ir.json.gz")
if err != nil { return err }

result, err := agentctx.Build(index, agentctx.Options{
    Root: repoRoot,
    Query: task,
    Depth: 1,
    MaxBytes: 32_768,
    MaxSymbols: 12,
    DenyPaths: []string{"secrets"},
})
if err != nil { return err }
// Supply result.Payload as a tool result. Do not re-encode or pretty-print it
// after counting; wrapper/message overhead must be budgeted by the harness.
```

For an **exact model-token cap**, set both `MaxTokens` and `CountTokens` in the
Go API. `CountTokens` must apply the target tokenizer to the final serialized
payload. An exact cap without a callback is rejected. Approximate counts never
satisfy an exact token budget. Reserve system, tool-schema, history, wrapper and
output tokens separately.

See `examples/agent/tool_bridge.py` for a model-neutral Python adapter that keeps
the binary, root, index, exclusion policy and output cap under application
control. It does not call any model service.

## Source and trust boundaries

- Source SHA-256 is checked for **every permitted indexed file**, including
  files not selected for this turn. Changed/deleted indexed files fail.
- This is not a Git snapshot manager. New files, ignored files and build/config
  changes are not detected. Recompile after repository changes and use a pinned,
  immutable worktree. Individual verified reads are not an atomic FS snapshot.
- A digest establishes consistency, not authenticity. The application must trust
  or authenticate the index and restrict the worktree and the executable.
- On Linux, descriptor-relative `openat` plus `O_NOFOLLOW` rejects symlink
  components and non-regular files. The non-Linux Go 1.23 fallback requires an
  immutable, access-controlled root; it is not race-hardened against simultaneous
  directory replacement. Neither implementation is a full sandbox.
- Native Tree-sitter parsing uses pinned, maintained grammar modules and bounded
  source/node/text lowering. It parses source without importing or executing
  repository modules, and has a per-file parse timeout. Syntax errors produce
  file diagnostics rather than semantic claims; source materialization requires
  UTF-8. HTML script/style contents are represented by the HTML grammar and are
  not recursively parsed as JavaScript/CSS.
- Markdown fenced-code bodies are likewise raw source context only. A fence's
  info string is not an embedded-language promise, and declarations, calls, and
  imports inside a fence are not indexed.
- Repository text remains untrusted, even when fenced or schema-valid. Labels
  and Markdown escaping do not solve prompt injection or enforce permissions.
- No repository build/test commands are inferred or executed. No test success
  or proof of behavior is claimed by a context bundle.

## Semantic limitations

Go, Python, HTML, CSS, JavaScript, TypeScript, TSX/JSX, and Markdown syntax ASTs are supported. `DEFINES`,
`IMPORTS`, and conservative `CALLS` are emitted where the grammar exposes a
source-linked construct. Markdown links and images emit conservative
`REFERENCES` occurrence edges, which remain unavailable as a context traversal
relation. Call links use same-language, same-unit name heuristics, **not
type-checked dispatch**. HTML
tags, CSS selectors/properties, and JavaScript/Python/TypeScript declarations are indexed as
syntax-level names; TSX/JSX support is syntax-only and does not claim React runtime,
component, type-resolution, or module-resolution semantics. This is not a claim about browser/runtime behavior. Interfaces,
inheritance, field usage, full import/alias resolution, embedded language
semantics, test coverage, data flow, build targets, architecture documents and
policy graphs are not implemented. Missing edges do not establish absence. Graph
neighbors and relation sites are candidates for inspection, not proof of
causality.

Go physical spans ignore `//line` redirection. Cross-file Go receivers and
simple generic receivers are linked within the package. Python decorators,
nested function/class scopes and assignment declaration ranges are preserved.
Repeated names require collision suffixes; semantic IDs are not immutable under
arbitrary edits. Each context reference is meaningful together with its snapshot.

## Project layout

```text
pkg/ir/                 compact wire IR, CSR and structural validation
pkg/compiler/           repository discovery, AST linking, validated IR I/O
pkg/agentctx/           task selection, source verification, bundles and rendering
internal/lang/goast/    Go AST front end
internal/lang/treeast/  pinned Tree-sitter Python/HTML/CSS/JavaScript/TypeScript/TSX/Markdown front end
internal/lang/pyast/    compatibility wrapper for the Python front end
internal/sourceroot/    bounded, root-relative source reads
main.go                 module-root compile / stats / graph / validate / context command
internal/cli/           shared command implementation
cmd/repoctx/            compatibility entry point for the same command
examples/agent/         application-side tool adapter
```

Documentation: [agent handoff contract](docs/AGENT_CONTEXT.md),
[IR reference](docs/IR.md), [IR schema](docs/ir.schema.json),
[context schema](docs/context.schema.json).

This release has automated contract and regression tests. It has **not** been
measured in a live coding-agent task-success evaluation; improved retrieval or
coding success is not claimed.
