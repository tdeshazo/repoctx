# Validation record — repoctx agent context edition

Environment: Go 1.23.2, native C compiler for Tree-sitter, Linux amd64. Python
is not required for indexing or tests.

- `go test ./...`: passed (including Tree-sitter language discovery, Markdown
  block/inline lowering, fenced-code raw-context coverage and malformed-input
  coverage).
- `go test -race ./...`: passed.
- `go vet ./...`: passed.
- `go build -buildvcs=false -trimpath .` and `go build -buildvcs=false -trimpath
  ./cmd/repoctx`: passed with the repository's empty placeholder `.git`
  directory.
- Go IR-validation fuzz smoke test: passed, 22,072 executions in a two-second run,
  one worker. This is a smoke test, not a security assurance claim.
- Draft 2020-12 JSON Schema validation: passed for mixed and self-compiled IRs,
  and for both corresponding agent JSON bundles.
- Every delivered JSON evidence span: exact bytes and SHA-256 checked against source.
- Full payload sizes, including escaping and final newline: checked against caps.
- Existing output preservation on CLI budget failure: passed.
- Legacy v1alpha2 index inspection succeeds; context serving correctly asks to recompile.
- Included Python tool adapter and snapshot-pinned expansion: executed successfully.

## Delivered examples

| Case | Symbols | Evidence blocks | Relationships | Rendered JSON bytes | Cap |
|---|---:|---:|---:|---:|---:|
| Mixed Go/Python query | 4 | 3 | 10 | 7250 | 20,000 |
| Compiler's own Build function | 2 | 3 | 9 | 15866 | 16,000 |

Mixed Markdown view: 10898 bytes within a 20,000-byte cap.
These are payload-size observations, not retrieval-quality or agent-success benchmarks.

## Self-compiled index

```
IR: repoctx.ir/v1alpha3
files: 30 (go=28 python=2)
nodes: 23197
symbols: 280
occurrence edges: 1478
graph nodes: 521
graph arcs: 1137
diagnostics: 0
strings: 2296
```

## Package-level test coverage

```
ok  	github.com/tdeshazo/repoctx/cmd/repoctx	0.017s	coverage: 4.5% of statements
		github.com/tdeshazo/repoctx/examples/mixed		coverage: 0.0% of statements
ok  	github.com/tdeshazo/repoctx/internal/lang/goast	(cached)	coverage: 75.6% of statements
ok  	github.com/tdeshazo/repoctx/internal/lang/pyast	(cached)	coverage: 71.4% of statements
		github.com/tdeshazo/repoctx/internal/sourceroot		coverage: 0.0% of statements
ok  	github.com/tdeshazo/repoctx/pkg/agentctx	0.354s	coverage: 84.0% of statements
ok  	github.com/tdeshazo/repoctx/pkg/compiler	0.439s	coverage: 82.0% of statements
ok  	github.com/tdeshazo/repoctx/pkg/ir	0.026s	coverage: 48.0% of statements
```

Coverage above is per-package (not cross-package coverage); source-reader behavior
is exercised through context/compiler tests. CLI end-to-end/schema/adapter checks
were run separately from the Go coverage collection. No claim of full coverage.

## Important scope limits

No live LLM or coding-agent benchmark was run. These tests establish interface,
source-fidelity and resource-bound behavior, not improved task success or general
prompt-injection resistance. SHA-256 freshness is limited to permitted indexed
files; new sources and unindexed build configuration require recompilation.
The index must be trusted/authenticated and the worktree immutable and scoped.

## Clean archive verification

The source ZIP was extracted to a new directory. `go test ./...`, `go vet ./...`,
and `go build -trimpath` passed from that extraction. The rebuilt executable was
byte-identical to the delivered Linux binary. The self-compiled snapshot digest
and mixed-example context payload were also identical after relocation.

Binary SHA-256:
`701e3ebd9251e279775dc79d29ae97b8541a88376ea4bdf92470978a2baf1a00`
