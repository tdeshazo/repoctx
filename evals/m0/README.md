# M0 deterministic evaluation corpus

This is a synthetic, public engineering corpus, not a statistically sufficient
benchmark. It contains 12 distinct tasks: four documentation, four code, two
mixed code/document, one absent-answer, and one access-restricted task. It
contains no private source or account configuration. The restricted value is a
synthetic marker used only to detect accidental disclosure.

## Layout and contract

- `manifest.json` lists task IDs, categories, and separate record paths.
- `sources/` holds exact fixture bytes. Go files use `.go.txt` storage names to
  keep them out of this project's Go test discovery.
- `tasks/` is evaluator-only: questions, source-to-repository path mappings,
  answer spans, expected relationships, scope, and budgets.
- `expected/` is evaluator-only: inspectable expected answers or abstention
  outcomes, references to required spans, and matching relationship records.
- `agent_inputs/` contains only the question, fixture ID, and permitted file
  inventory. Source content comes from the exported repository, not placeholder
  prose or hand-selected answer excerpts.
- `scoring.json` defines evidence, relationship, scope, budget, and answer
  scoring separately, including null/not-applicable and unavailable cases.

The v2 manifest replaces the earlier placeholder corpus. In each task,
`source_map` maps stored source filenames to actual repository paths such as
`src/retry.go`. Scope uses these repository paths with component-prefix
matching; deny wins. Span offsets are zero-based, half-open UTF-8 byte ranges
in the stored file, which is copied byte-for-byte to its mapped path. Expected
span IDs must match the task's spans, and duplicated relationship records must
agree. Every relationship cites supporting span IDs.

Only actual import/call occurrences have relationship obligations. A variable
read is not a call, and ordinary prose is not a syntactic reference edge.
The Go import fixture explicitly evaluates the **index** surface; package-level
Go imports are not currently emitted as public context relationships. Empty
relationship lists mean no relationship scoring obligation, not absence of
relationships in the source. Mixed tasks require both document and code spans.

## Validate and replay

Validate the entire corpus and run its regression tests:

```sh
python3 scripts/check_fixtures.py
python3 -m unittest discover -s tests -p 'test_m0_fixtures.py' -v
```

The checker verifies byte ranges/text, expected-record agreement, source
mapping, scope, budgets, relationship evidence references, evaluator/agent
separation, distinctness, and category minimums. Malformed records produce a
machine-readable failure with exit status 1. It does not prove the semantic
correctness of an answer: the small source files and expected answers remain
independently inspectable.

Export one fixture to a new directory:

```sh
fixture_run=$(mktemp -d)
python3 scripts/check_fixtures.py --fixture code-import-sites \
  --output "$fixture_run/trial"
```

Give the agent only `trial/input.json` and `trial/repository/`. Keep this corpus
directory and all evaluator records inaccessible to the agent. The exporter
copies all permitted source bytes with real extensions, omits denied files,
and refuses an existing output directory. It does not compile source, invoke
an agent, or run repository commands. A harness can compile the exported
repository and submit the original question to repoctx; M0 does not assert that
the current selector retrieves every answer-bearing span. Scope and budget
enforcement remains the caller's responsibility when running retrieval trials.

The import fixture also includes a local `go.mod` and store package so its
repository-module import has an inspectable target. The roadmap fixture has
explicit priority, completed criteria, and the next unmet criterion. The absent
fixture provides a neutral module inventory; its agent input does not reveal
the expected no-answer judgment.

Repeating an ID or relabeling an identical question/source pair does not create
a distinct task. Category counts are minimums, so future unique tasks can be
added without changing the original target. Corpus validity, fixture coverage,
test coverage, retrieval quality, and agent outcomes are separate measurements.
Passing this checker is not a retrieval-quality or live-agent outcome result.
