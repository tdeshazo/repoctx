# repoctx — repository index and agent context compiler

`repoctx` supports index-free repository discovery and an indexed evidence
workflow. The indexed workflow has **two outputs with different consumers**:

1. **Repository IR (`repoctx.ir/v1alpha4`)**: compact Go/Python/HTML/CSS/JavaScript/TypeScript/TSX/Markdown ASTs, interned
   strings, symbols, occurrence edges, and dense forward/reverse CSR adjacency.
   This is the reusable machine index, not the prompt.
2. **Agent context (`repoctx.context/v1alpha3`)**: a selected, readable JSON or
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

Inspect the executable before relying on commands or wire-contract versions:

```sh
repoctx version
repoctx version -format json
```

The JSON form uses `repoctx.build/v1alpha1` and reports the release, source
revision and modified state when the Go toolchain supplied them, distribution,
Go/platform versions, and supported IR/context/discovery/artifact contracts.
Local `go run` and builds made with `-buildvcs=false` can report `unknown`
revision state. Version output is descriptive metadata, not authentication or
proof that a binary matches a checkout; compare it with a caller-trusted release
or source revision when that distinction matters.

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

The Go build uses CGO for Tree-sitter. A native C compiler and the Go toolchain
are required when building from a checkout or source distribution; set `CC` (and
any platform-specific compiler flags) in the environment when the compiler is
not the platform default. `CGO_ENABLED=0` is not supported.

`.ts`, `.mts`, and `.cts` files use the TypeScript grammar; `.tsx` and `.jsx`
files use the TSX grammar. `.js`, `.mjs`, and `.cjs` remain JavaScript and are
parsed by the separate JavaScript grammar.
`.md` files use the maintained Markdown block and inline grammars. Headings and
conservative link relationships are source-linked. Fenced-code bodies remain
raw Markdown context only and are never recursively parsed or treated as
embedded Go, TypeScript, or another language.

## Python package

The `repoctx` distribution provides the same module-root Go command through a
console script and `python -m repoctx_cli`. A wheel install does not need Go or
a C compiler at runtime. To install from this checkout:

```bash
python -m pip install .
repoctx --help
python -m repoctx_cli --help
```

To build and install from this checkout, use Python 3.9 or newer with
`setuptools`, `wheel`, and the `build` frontend available:

```bash
python -m pip install .
python -m build --wheel
python -m build --sdist
python -m pip install dist/repoctx-*.whl
```

If the `build` frontend is not installed, `python setup.py bdist_wheel` is
also supported for local builds.

Building a wheel or installing directly from this checkout compiles the Go
module with CGO. Building from an sdist likewise requires Go 1.23 or newer and
a working C compiler; the sdist contains the Go sources and pinned module
checksums needed for that build. Build on the target platform: the wheel is
platform-specific and is deliberately not tagged as `py3-none-any`. Only the
Linux amd64 build is validated in this repository; other platform builds are
not claimed unless their Go, CGO, and Tree-sitter toolchains succeed.

The Python launcher only forwards arguments and inherited standard streams to
the bundled executable. It does not reimplement indexing, and its Markdown
fence behavior is therefore exactly the Go CLI's raw-context policy described
above. Python-built binaries report the Python project release and
`python-package` distribution through `repoctx version`; source revision remains
`unknown` because package metadata alone does not authenticate source identity.

## Agent skill

The vendored [repoctx skill](skills/repoctx/SKILL.md) teaches agents to compile
an index, retrieve bounded evidence, check executable provenance, check freshness
and limitations, and expand context only when needed.

Copy the `skills/repoctx` directory into your agent's skill directory to use it
outside this checkout. Installing the Go binary does not install the skill.

## Repository discovery without an index

Discovery combines repository orientation, file finding, text matching, and
exact excerpts. It does not require Git, ripgrep, an index, or a model API, and
creates no cache. Use the combined command first when you need several of these
operations; use focused commands for a known follow-up:

```sh
repoctx discover -root . -query 'retry configuration' -max-bytes 12000
repoctx overview -root . -depth 2
repoctx files -root . -glob '*.go'
repoctx files -root . -type directory -glob '*test*'
repoctx search -root . -query 'MaxAttempts' -context-lines 3
repoctx search -root . -query '^func .*Retry' -regex -ignore-case
repoctx read -root . -file README.md:1:30 -file go.mod
```

All arguments are named flags; `COMMAND -help` lists them. `-root` defaults to
the current directory, never a parent Git root. `read -file PATH:START:END`
uses inclusive one-based lines, with zero meaning the beginning/end. Repeated
requests share file reads and overlapping or adjacent ranges are merged.
An out-of-range line request is a usage error, not a silently shortened read.

Discovery inventories regular files regardless of parser support, including
YAML, TOML, and plain text. It honors nested repository-local `.gitignore` and
`.ignore` rules; `.ignore` rules take precedence over `.gitignore`, and deeper
rules override shallower rules within each kind. Supported patterns include
negation, root anchoring, directory suffixes, escaped characters, character
ranges, and whole-component `**`. An excluded parent directory is not traversed,
so a child negation cannot reopen it. Global Git configuration, `.git/info/exclude`,
and tracked status are not consulted; this is not exact `git ls-files` parity.

Hidden entries are excluded unless `-hidden` is supplied. `-no-ignore` disables
ignore rules independently of hidden visibility. Neither option bypasses
`-allow`/`-deny` prefixes, and `.git` internals are always excluded. Scope applies
to ignore-file reads too: unavailable rules are reported rather than read outside
scope. Explicit `read` requests use the same visibility policy. Symlinks and
special files are never followed/opened as content. There is no secret scanner.

`search` performs literal, case-sensitive, line-by-line matching by default;
`-regex` enables Go regular expressions. Slashless `-glob` patterns match file
basenames; patterns containing `/` match root-relative paths. `discover` splits
task text into distinct lowercase Unicode letter/digit terms and matches
substrings in paths and source. Windows rank by distinct matched terms, path
terms, and source occurrences capped at ten per term, then path and byte offset.
It performs no stemming, embeddings, or query expansion. The returned score is
not confidence or a guarantee of answer sufficiency. Filename-based documentation
and configuration labels are inspection leads, not authoritative entry points.

JSON is the default, with the separate
[`repoctx.discovery/v1alpha1` contract](docs/discovery.schema.json).
`-format markdown` renders the same records as indented JSON for safe review;
source text is JSON-escaped and round-trips exactly. Each excerpt includes the
observed file hash, inclusive lines, and half-open UTF-8 byte offsets. Existing
IR and context schemas are unchanged.

Default bounds are 32 KiB final output (`-max-bytes`), 2 MiB per file
(`-max-source-bytes`), 256 MiB total content including ignore files
(`-max-read-bytes`), 100 results (`-max-results`), and 100,000 enumerated entries
(`-max-entries`). Overview depth defaults to two; it limits presentation, not
the inventory scan. Query-bearing discovery includes at most 12 overview entries
to leave room for excerpts. Both formats enforce their own final serialized
byte bound, including escaping and metadata, not a token bound.

Check `incomplete`, `omissions`, and `warnings`. Binary, non-UTF-8, oversized,
unreadable, and unsafe content is explicitly omitted; inventory-only commands
do not open content to classify it. Oversized evidence records are omitted whole,
never silently turned into declaration prefixes. Scan, read, result, overview,
and output limits have distinct reasons. If a directory exceeds the remaining
scan budget, the retained subset depends on filesystem enumeration order; complete
scans have deterministic ordering. Live results are not an atomic snapshot:
concurrent writers can change the tree between reads. Linux uses descriptor-relative
no-symlink access; other platforms require an immutable, access-controlled tree.

Empty searches succeed with an empty list. Invalid options/ranges exit 2;
execution failures exit 1. Diagnostics go to stderr and payloads to stdout, or
an explicitly requested `-o` file written only after successful serialization.
Discovery complements ordinary shell tools; no reduced-call or task-success
improvement is claimed until a paired agent evaluation measures it.

## Compile, then retrieve for an agent

For initial navigation without an index, start with the
[discovery toolkit](#repository-discovery-without-an-index). Compile when you
need indexed symbols and relationships.

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

Caller-controlled file/directory prefixes exclude source and auxiliary inputs
before indexing. Explicit serving scope must match the index's compilation scope:

```sh
./repoctx compile -root /repo -allow services/payments \
  -deny services/payments/private -o repo.ir.json.gz
./repoctx context -root /repo -query 'retry' \
  -allow services/payments -deny services/payments/private repo.ir.json.gz
```

These flags take prefixes, not glob patterns. Deny wins. Omit serving flags to
use the authenticated index's declared policy; recompile for a different policy.
Indexes contain source-derived literals
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

Context now searches verified document bodies, comments, and source literals
alongside symbol names and paths. Markdown documents, sections, paragraphs,
checklist/list items, tables, and code blocks have separate retrieval units;
heading symbol spans remain unchanged. Use a returned unit ID for expansion:

```sh
repoctx context -root /repo -unit 'u:ID_FROM_A_PREVIOUS_BUNDLE' \
  -expect-snapshot 'sha256:PREVIOUS_SNAPSHOT' -max-units 8 /tmp/repo.ir.json.gz
```

Units are derived in memory after input verification, not persisted in a text
cache. M2 adds a required compilation-input manifest in IR v1alpha4 and separate
source/profile/index/task identities in context v1alpha3. Recompile older indexes.
Consumers must also accept `units`, unit IDs in `seeds`, ranking components, and
`query_excerpt` completeness. The
[protocol and migration notes](docs/AGENT_CONTEXT.md) describe the contract;
[the v1alpha1 schema](docs/context-v1alpha1.schema.json) remains for old artifacts.

The planner uses deterministic lexical seeds and bounded graph expansion. It
reserves up to a quarter of the byte budget (capped at 4096 bytes) for supporting
imports and graph evidence. It does not expand through package hubs or globally
merged unresolved names. Oversized definitions become **explicitly marked exact
source excerpts**, preferring query-centered windows and separate declaration
leads where feasible, not invented summaries. A required seed that cannot fit causes
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
payload, and `TokenizerID` must identify that implementation/version. An exact cap
without a callback is rejected. Approximate counts never
satisfy an exact token budget. Reserve system, tool-schema, history, wrapper and
output tokens separately.

See `examples/agent/tool_bridge.py` for a model-neutral Python adapter that keeps
the binary, root, index, exclusion policy and output cap under application
control. It does not call any model service.

## Source and trust boundaries

- Default `verified-local` mode checks the declared inventory and all source and
  auxiliary hashes, including nonselected files and absent optional `go.mod`.
  Added/deleted/renamed/modified permitted source and `go.mod` changes fail.
- Compilation does not honor ignore files or build tags; it records fixed directory
  exclusions, caller scope, limits and syntax-only compiler/frontend identities.
  Denied `go.mod` is never read; module metadata is explicitly unavailable.
- `-consistency immutable -expect-snapshot sha256:...` reuses an authenticated
  manifest under a caller-owned immutable-tree guarantee; source hashes still get
  checked. Dirty/untracked permitted files count as inputs in either mode. Checks
  are not an atomic filesystem snapshot; callers must isolate concurrent writers.
- Compilation caps per-file bytes, total bytes across both passes, and inventory
  entries, and atomically publishes complete file outputs. See the
  [input and consistency contract](docs/AGENT_CONTEXT.md#replay-source-changes-and-policy).
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
[context schema](docs/context.schema.json),
[evaluation report and latest results](docs/reports/mothership-repoctx-codex-exec-comparison.md).

This release has automated contract and regression tests. It has **not** been
measured in a live coding-agent task-success evaluation; improved retrieval or
coding success is not claimed.

## License

Copyright 2026 Travis DeShazo. Licensed under the
[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) for attribution.
