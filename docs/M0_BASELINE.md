# M0 baseline and evaluation contract

M0 is the reproducibility gate. It makes correctness, artifact validity,
retrieval quality, and agent outcomes separate measurements; it does not claim
that the 12-fixture corpus is statistically sufficient.

## Reproduce from a clean checkout

Run from the repository root with a declared Linux amd64 environment, Go 1.23
or newer, a native C compiler, and Python 3.9 or newer:

```sh
python3 scripts/m0_baseline.py --output /tmp/repoctx-m0-baseline.json
```

The runner records `git rev-parse HEAD`, clean/dirty status, a source-tree
digest, Go/C/Python toolchains, platform, pinned Tree-sitter grammar lines from
`go.mod`, every command, exit code, duration, and bounded stdout/stderr tails.
Failures remain in the JSON report; independent checks continue after a
failure. The report is a generated run artifact and should be retained with
the revision under evaluation, not silently replaced by historical coverage
numbers.

The command sequence is:

1. `go test ./...`
2. `go test -race ./...`
3. `go vet ./...`
4. `go build -buildvcs=false -trimpath -o <temporary> .`
5. `go build -buildvcs=false -trimpath -o <temporary> ./cmd/repoctx`
6. Compile, validate, and render the mixed example with the fresh binary.
7. Check the IR and context JSON artifacts against `docs/ir.schema.json` and
   `docs/context.schema.json`.
8. Build a wheel and sdist with `python -m build --no-isolation`, install each
   into a temporary target, and run `python -m repoctx_cli --help` from each
   installed target.
9. Check the 12 deterministic fixtures with `scripts/check_fixtures.py`.
10. Run the Python launcher tests and a source-tree launcher smoke check.

The runner uses a temporary Go build cache and the caller's declared Go module
cache; it never runs repository build, test, plugin, or shell commands
discovered from source. Distribution builds use the supplied test environment
with `--no-isolation`; they do not contact an account or install undeclared
dependencies. Full Draft 2020-12 validation requires `jsonschema`:
The baseline declares its Python test dependencies in `requirements-test.txt`
and the `test` optional extra. Missing `jsonschema` is a failed schema check;
there is no fallback subset that can be reported as full validation.

The Go contract tests exercised by the first three commands cover exact source
bytes, UTF-8 byte columns and CRLF positions, final JSON/Markdown bounds,
caller-tokenizer success and failure, overlapping evidence merging, changed
indexed sources, denied paths, and malformed index input. The tests remain with
their owning packages (`pkg/agentctx`, `pkg/compiler`, and `pkg/ir`) so a report
can distinguish contract validity from fixture coverage and retrieval quality.

## Deterministic fixtures

The [v2 corpus and replay contract](../evals/m0/README.md) supersedes the
placeholder fixture records used by the retained M0 baseline report. Validate
the current corpus with `python3 scripts/check_fixtures.py`; that earlier report
is historical evidence and does not certify subsequent corpus edits.

`evals/m0/manifest.json` has exactly 12 unique IDs: four documentation, four
code, two mixed, and two no-answer/access-restricted tasks. For every fixture:

- `tasks/` stores the task, answer-bearing byte spans, expected relationships,
  permitted scope, and budgets;
- `expected/` stores the independently inspectable expected outcome; and
- `agent_inputs/` stores only the question, fixture ID, and permitted file
  inventory; the replay command exports the actual permitted source bytes.

`scoring.json` keeps scoring rules out of agent inputs. Repeating a task ID does
not increase the distinct count. No-answer fixtures require an explicit
absence or access-denied result; inventing a value is incorrect.

## Import and go.mod boundary

Go import edges preserve their AST occurrence nodes for both a repository
module (`example.test/sub`) and an external module (`fmt`). This graph-level
coverage records source-linked occurrences. Current public `Build` relationship
rendering is intentionally narrower: package-level Go imports are owned by a
unit node, while `Build` enumerates only arcs adjacent to selected symbols, so
it does not render those imports as bundle relationships. It can still retain
the selected file's import declaration as supporting evidence. Python
function-local imports are symbol-owned and exercise the public relationship
site path in regression tests. This is source-linked occurrence evidence, not
type/module resolution or runtime causality.

The compiler reads the root `go.mod` module name to construct repository-module
graph labels, but `go.mod` is not currently a discovered/indexed source file.
Changing it after compilation therefore does not invalidate an existing index
in M0; the focused test records this limitation explicitly. M2 is responsible
for a complete compilation-input manifest and scope-aware freshness. Callers
must recompile after build/config changes and must not treat the current index
as a whole-repository snapshot.

The retained [baseline report](reports/m0-baseline.json) records the clean
temporary snapshot revision, source-tree digest, platform, toolchains, and
pinned grammar dependencies for its own run. Its four measurements deliberately
mark retrieval quality and agent outcomes as `not_measured`.
