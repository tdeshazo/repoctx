# repoctx — repository index and agent context compiler

`repoctx` supports index-free repository discovery and an indexed evidence
workflow. The indexed workflow has **two outputs with different consumers**:

1. **Repository IR (`repoctx.ir/v1alpha5`)**: compact Go/Python/HTML/CSS/JavaScript/TypeScript/TSX/Markdown ASTs, interned
   strings, symbols, occurrence edges, and dense forward/reverse CSR adjacency.
   An explicit `-manifest` compile may attach typed, exact-source semantic
   declarations. Without it, the IR remains source-only. This is the reusable
   machine index, not the prompt.
2. **Agent context (`repoctx.context/v1alpha3`)**: a selected, readable JSON or
   Markdown bundle with exact source, semantic IDs, provenance, selection
   reasons, bounded relationships, explicit omissions, and trust metadata.
   Pass this as **tool-result evidence**, not as system/developer instructions.

Applications may also opt into a separate **obligation handoff
(`repoctx.obligations/v1alpha1`)**. It binds caller-authorized contracts,
requirements, declared verification links, and opaque runner check IDs to an
existing context task without executing a check or manufacturing a result.

The examples below assume a current `repoctx` executable. Check it once with
`repoctx version -format json`; if that command is unavailable or reports a
different checkout, build the checkout with `go build -o repoctx .` and use
`./repoctx` in the examples, or reinstall the current version. Repeat the
check only when the executable may have changed.

## Default workflow

For an ordinary repository question, use the smallest path that answers it:

1. **Discover** the relevant files and excerpts without an index.
2. **Inspect and read** the returned paths, ranges, and completeness fields. If
   the evidence answers the task, stop; use focused `search` or `read`
   follow-ups only when more context is needed.
3. **Expand** with a compiled index only when symbols or relationships will
   improve the answer. Use the returned semantic IDs and snapshot for a
   targeted context follow-up.

```sh
repoctx discover -root . -query 'the task, error, or component' -max-bytes 12000
# Replace README.md:1:30 with a path and inclusive range from discovery.
repoctx read -root . -file README.md:1:30

# Only when indexed relationships help:
repoctx compile -root . -o /tmp/repoctx.ir.json.gz
repoctx context -root . -query 'the component or API' -depth 1 \
  -max-bytes 12000 /tmp/repoctx.ir.json.gz
```

Compilation is optional for orientation and focused source reads. Manifests,
artifact catalogs, persisted file-reference bundles, and checkpoints are
caller-controlled extensions; an ordinary lookup does not require authoring
metadata or configuring a session store.

When canonical semantic inputs are needed, a repository may declare them in a
strict, bounded [`agent-context.yaml`](agent-context.yaml). Validate an existing
manifest and print its normalized semantic model without compiling or activating
providers; do not author one for an ordinary lookup:

```sh
repoctx manifest -root . -file agent-context.yaml
repoctx compile -root . -manifest agent-context.yaml -o repo.ir.json.gz
```

See [the manifest contract](docs/MANIFEST.md) for the closed schema, limits,
availability checks, and authored-versus-generated boundary.

## Build and test

```sh
go test ./...
go vet ./...
go build -o repoctx .
go build -o repoctx-cmd ./cmd/repoctx
```

The authored [Usage specification](repoctx.usage.kdl) describes the complete
command, argument, flag, effect, and completion surface without generating a
second prose reference. With the `usage` CLI installed, validate or derive
artifacts from it:

```sh
usage lint repoctx.usage.kdl
usage generate markdown --file repoctx.usage.kdl --out-file /tmp/repoctx-cli.md
```

Inspect the executable before relying on commands or wire-contract versions:

```sh
repoctx version
repoctx version -format json
```

Artifact maintainers can generate exact hashes and source coordinates from a
compact semantic declaration instead of authoring deterministic fields:

```sh
repoctx artifacts -root . -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json
```

Use `-check` with the same arguments to detect stale output without writing it.
See [the artifact documentation](docs/ARTIFACTS.md#repository-dogfood-catalog).

The JSON form uses `repoctx.build/v1alpha5` and reports the release, source
revision and modified state when the Go toolchain supplied them, distribution,
Go/platform versions, and supported Go API, IR, context, discovery, artifact,
provider, obligation, diagnostic, projection, manifest, entity, and frontmatter contracts. See the [active-development contract
policy](docs/COMPATIBILITY.md) for current versions, strict field handling, and
rebuild requirements. Pre-1.0 contracts may break without a migration path.
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

The vendored [repoctx skill](skills/repoctx/SKILL.md) teaches agents to discover
and inspect bounded evidence first, compile an index when useful, check
executable provenance, check freshness and limitations, and expand context only
when needed.

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
substrings in paths and source. Punctuated fields that look like identifiers,
such as `M4-12` or `rcx.topic.retry`, remain intact: results with exact identifier
matches rank first while ordinary lexical matches remain eligible. Exact matches
must not be embedded in a larger letter/digit sequence, but punctuation remains
a delimiter so identifiers match prose, dotted symbols, and paths such as
`docs/M4-12-evidence.md`. Their source windows center on lines containing the
intact identifier. Ordinary hyphenated prose keeps the term-based behavior.
Discovery windows cannot grow beyond one requested context window by chaining
nearby weak matches. Windows then rank by distinct matched terms, declared
document metadata, evidence class for exact identifier matches, path terms,
source occurrences capped at ten per term, path, and byte offset.
For broad queries whose best window matches at least four terms, `discover`
builds an eight-result coverage prefix from matches with at least half the best
distinct-term score: the best documentation, configuration, and source result
first, then distinct files. Unselected windows retain their original order.
This keeps weak alternatives from displacing strong repeated evidence while
allowing bounded output to expose more than one evidence class and file.
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
Discovery complements ordinary shell tools. A small exploratory paired pilot is
documented in the [workflow pilot report](docs/reports/m4-13-workflow-pilot.md);
it did not establish an efficiency or task-success improvement and is not a
general usability claim.

## Expand with an index when relationships help

Start with the [discovery toolkit](#repository-discovery-without-an-index),
inspect its evidence, and read the useful paths or ranges. Compile when you
need indexed symbols and relationships; keep the generated index in a
caller-owned temporary location unless a durable artifact is required.

```sh
./repoctx compile -root examples/mixed -o /tmp/mixed.ir.json.gz
./repoctx validate /tmp/mixed.ir.json.gz

./repoctx context \
  -root examples/mixed \
  -query 'Worker.files' \
  -depth 1 \
  -max-bytes 20000 \
  -o /tmp/context.json \
  /tmp/mixed.ir.json.gz
```

Repeated compilation may opt into a caller-owned, content-addressed parse cache:

```sh
./repoctx compile -root examples/mixed \
  -cache-dir /tmp/repoctx-parse-cache -o /tmp/mixed.ir.json.gz
```

The cache must be a private real directory outside the indexed repository. It
contains source-derived names and literals, is not authenticated storage, and
must not be shared across callers with different trust scopes. repoctx validates
cached records and rebuilds all snapshot-local links and graphs; omit the flag
for a clean build. Cache statistics go to stderr, leaving stdout deterministic.

JSON is the default. Status and approximate token usage go to stderr, so stdout
contains only the payload. All flags must precede the final index argument.
`-max-bytes` limits the **whole rendered, uncompressed output**, including JSON
escaping, metadata and the trailing newline. It is not a model token limit.

```sh
./repoctx context -root examples/mixed \
  -symbol 'py:pkg.worker#Worker.files' \
  -format markdown -max-bytes 20000 /tmp/mixed.ir.json.gz
```

Do not send the raw CSR/string-table index to the model. Do not infer a token
count from gzip size. The CLI's bytes/4 token estimate is explicitly approximate.

## Incremental disclosure and scope

Take `snapshot.id` and a semantic ID from a returned symbol or relationship:

```sh
./repoctx context -root examples/mixed \
  -symbol 'py:pkg.worker#Worker.files' \
  -expect-snapshot 'sha256:THE_DIGEST_FROM_THE_PREVIOUS_BUNDLE' \
  -direction out -depth 2 -max-bytes 12000 /tmp/mixed.ir.json.gz
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
cache. M2 adds a required compilation-input manifest in IR v1alpha5 and separate
source/profile/index/task identities in context v1alpha3. Recompile older indexes.
Current consumers must handle `units`, unit IDs in `seeds`, ranking components,
and `query_excerpt` completeness. The [agent context contract](docs/AGENT_CONTEXT.md)
describes the current protocol. Older schema files are historical development
records, not supported reader targets.

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
control. It does not call any model service. Inline delivery remains the
default. Persisted file-reference delivery and checkpoints are optional. Callers
may instead configure a caller-scoped bundle directory outside
the indexed repository, supply a unique `run_id`, and request
`delivery="file_reference"` with their own opaque, run-unique handle. The
response contains snapshot identity, exact byte size and digest, relevant
symbol/unit IDs, trust, omissions, expiry, and bounded-read limits; it does not
expose a filesystem path or substitute a preview for evidence.

`read_context(handle, offset=..., max_bytes=...)` returns an integrity-checked,
base64 byte range whose source-byte and serialized-response sizes are both
bounded. Concatenating decoded ranges reconstructs the exact original context
payload, including ranges that divide a UTF-8 character. The adapter rejects
unknown/reused handles, changed stored bytes, and bundle directories within the
source root. Run directories and bundles are owner-only; bundle publication is
atomic and subject to caller-set count, byte, and retention limits.

After persisted retrieval, `create_checkpoint(...)` returns bounded JSON with
the task, run and bundle handles, snapshot, relevant IDs, unresolved questions,
and next retrieval. The harness owns this checkpoint and must keep it outside
the indexed repository. To resume, construct the adapter for the same run with
`resume_run=True`, then call `resume_checkpoint(...)`. It verifies every stored
bundle and replays a snapshot-pinned context request before enabling reads.
`cleanup_expired()` only removes expired bundles registered to that run; missing,
expired, cross-run, and stale-snapshot failures are explicit.

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
- Repeated Go API queries can call `agentctx.VerifySourceGeneration` once, then
  `agentctx.BuildFromGeneration` with its authenticated `SnapshotID`. The
  generation privately owns a bounded index copy and exact source bytes that
  passed strict local verification, so later queries perform no filesystem reads.
  Retention and authorization remain application-owned; the one-shot CLI keeps
  strict local verification available and does not create a hidden persistent cache.
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

This release has automated contract and regression tests. A small exploratory
paired pilot completed four synthetic tasks with both conditions at 4/4 and did
not establish an efficiency or coding-success improvement. See the [workflow
pilot report](docs/reports/m4-13-workflow-pilot.md); it is not a confirmatory
live-agent evaluation or a general improvement claim.

## Maintenance and security

See the [changelog](CHANGELOG.md) for user-visible releases, the
[contributor guide](CONTRIBUTING.md) for development expectations, and the
[release procedure](docs/RELEASING.md) for reproducible Linux amd64 artifacts,
checksums, and unsigned provenance. Runtime dependency attribution is in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
Do not place exploit details, credentials, or private repository content in a
public issue.

## License

Copyright 2026 Travis DeShazo. Licensed under the
[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) and
[third-party notices](THIRD_PARTY_NOTICES.md) for attribution.
