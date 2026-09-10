# M0 fixture replay — gpt-5.6-terra

Executed September 9, 2026. **11 of 12 distinct tasks passed the strict stored
answer rubric (91.7%).** All 12 model processes completed successfully, all
citations were exact source quotations, and both abstention tasks passed.

This is a **full-permitted-source replay** of the M0 v2 corpus: each fresh
model session received the original question and every permitted source file.
There was one trial per task, no answer retries, and no answer-informed source
selection. These results measure answering with supplied source. Repoctx
retrieval, autonomous repository navigation, coding outcomes, and improvement
over another model or condition were not measured.

## Results

| Fixture | Outcome | Answer assessment | Evidence payload | Seconds |
| --- | --- | --- | ---: | ---: |
| [doc-roadmap-priority](pilot/doc-roadmap-priority/answer.txt) | Pass | Evidence bundles; next unmet criterion is CRLF byte-offset preservation | 363 B | 6.386 |
| [doc-api-contract](batch/doc-api-contract/answer.txt) | Pass | Syntax-only compilation; no repository command execution | 287 B | 5.759 |
| [doc-config-boundary](batch/doc-config-boundary/answer.txt) | Pass | Unindexed build configuration, including go.mod and tsconfig.json; recompilation appears in the citation | 322 B | 5.642 |
| [doc-release-notes](batch/doc-release-notes/answer.txt) | **Fail: incomplete** | Correct descriptive/not-task-success limitation, but omits the single-run condition | 313 B | 5.028 |
| [code-import-sites](batch/code-import-sites/answer.txt) | Pass | Identifies src/imports.go, example.test/store, and fmt | 469 B | 4.910 |
| [code-retry-policy](batch/code-retry-policy/answer.txt) | Pass | Three attempts, citing maxAttempts | 220 B | 4.707 |
| [code-auth-boundary](batch/code-auth-boundary/answer.txt) | Pass | load_profile, with quoted verify_token-before-profile sequence | 405 B | 5.314 |
| [code-parser-error](batch/code-parser-error/answer.txt) | Pass | Returns "invalid" for empty input | 219 B | 4.758 |
| [mixed-release-gate](batch/mixed-release-gate/answer.txt) | Pass | ValidateRelease checks report.Reproducible; cites document and code | 527 B | 5.672 |
| [mixed-schema-owner](batch/mixed-schema-owner/answer.txt) | Pass | ContextVersion in src/context.go, with the exact schema version in its citation | 456 B | 4.644 |
| [no-answer-restricted](batch/no-answer-restricted/answer.txt) | Pass | Explicit access_denied/unavailable result, without disclosure or guessing | 299 B | 4.915 |
| [no-answer-absent](batch/no-answer-absent/answer.txt) | Pass | Explicitly says supplied sources identify no such module | 228 B | 5.071 |

Category results: documentation **3/4**, code **4/4**, mixed **2/2**,
no-answer/access-restricted **2/2**. Overall is a task-weighted proportion;
repeated trials did not increase any denominator.

The sole failure is rubric completeness, not a fabricated claim. Its response
states the descriptive and task-success limitations, but omits the expected
single-run qualification from both the answer and quoted evidence. The
[stored expectation](../../../evals/m0/expected/doc-release-notes.json) requires
that qualification. No follow-up was used to repair the answer.

## Measurement boundaries

| Measurement | Result | Interpretation |
| --- | --- | --- |
| Fixture coverage | 12 distinct tasks; required 4/4/2/2 category split | Engineering smoke corpus |
| Execution | 12/12 successful processes and parseable answers | No model-execution failures |
| Agent outcomes | 11/12 strict rubric passes | Manual semantic grading by the primary assistant |
| Outcome labels | 12/12 match answerable/no_answer/access_denied | Correct label alone does not establish complete answer content |
| Citation validity | 12/12 responses use exact quotes from supplied files | Mechanically verified; semantic support also inspected |
| Source/scope validity | 12/12 exact permitted-source deliveries; denied source excluded | No tool calls observed; no denied value in answers |
| Delivered answer-span coverage | 15/15 required spans across 10 answerable tasks | Full-source delivery makes this 100%; it is not retrieval performance |
| Relationship recall | Unavailable for 3 obligations across 2 tasks | No index/context relationship surface was produced in this condition |
| Other relationship cases | Not applicable for 10 tasks | Empty obligations are not scored as successes |
| Evidence byte budgets | 12/12 within task caps | Final compact JSON payloads are 219–527 bytes; caps are 12,000 or 16,000 bytes |
| Symbol/token caps | Symbol selection not applicable; exact token cap not claimed | No symbols selected; fixture max_tokens values are null |
| Test coverage and software outcomes | Not measured in these trials | Corpus validation is separate from program test coverage |

Scoring follows the corpus [rules](../../../evals/m0/scoring.json). A response
includes its answer text and exact quoted citations: required claims explicitly
present in either can satisfy completeness. This rule was applied consistently;
for example, the authentication sequence and schema version appear in cited
code. The [manual judgments](judgments.json) state each decision and rationale.
This is not independent human grading or a blinded model comparison.

## Execution conditions and provenance

- Requested model: **gpt-5.6-terra**, explicitly selected for every invocation;
  reasoning effort: **medium**. No model substitution was used. The CLI events
  did not expose an immutable served-model snapshot; that field remains null.
- Runner: **codex-cli 0.153.4**; fresh ephemeral session per task. A pilot on the
  roadmap task was retained as trial 1; the other 11 tasks ran with up to three
  concurrent processes. The pilot's approval review initially timed out before
  a process started; the permitted approval retry is not a model trial.
- The [runner](run.py) used read-only mode, ignored user configuration, disabled
  project instruction loading, host skill discovery, shell execution, apps,
  plugins, web search, and delegation features. It passed only the question
  and exported permitted source as task data. No evaluator answers, spans,
  scoring rules, or denied file contents were included in model prompts.
  The runner settings were checked against the local CLI help and the official
  [CLI reference](https://developers.openai.com/codex/cli/reference/) and
  [configuration reference](https://developers.openai.com/codex/config-reference/).
- Base repository revision: `f6bf293213bf1e16988076075929cfbe84eae819`.
  **The worktree was dirty**: this base commit alone does not identify the
  evaluated v2 corpus. Per-file SHA-256 values are retained in both run manifests.
- Corpus digest: `d4cc4e56ef6e73fc128dc61ac7e352318716caf875c54ac4288f095dc2da1c30`.
  The pilot and batch digests agree. The report compiler verifies current corpus
  files against those recorded hashes before grading.
- Initial trial timestamp: `2026-09-09T12:02:43.884521+00:00`.
  See [pilot metadata](pilot/run.json) and [batch metadata](batch/run.json).

The original prompts, delivered evidence, final answers, usage, and sanitized
execution events are retained under each fixture directory. Reasoning events
and account configuration are not included. Exact prompts include a common
instruction to answer only from supplied files and return a JSON answer with
verbatim source citations. Those instructions are identical across tasks and
contain no expected answers.

Reported usage totals: **154,667 input tokens**, including **69,120 cached input
tokens**, and **915 output tokens**. The CLI also reported **109 reasoning output
tokens**, retained as a separate field without adding it to output totals.
Input usage includes the CLI's system/developer overhead and therefore exceeds
the small fixture payload sizes. No cost estimate is made.

Median per-process elapsed time was **5.050 seconds**. The sum was **62.806
seconds**; concurrent execution means this sum is not end-to-end wall time.
These are single-run observations, not a controlled latency benchmark.

## Reproduce and inspect

Recompile the report's machine-readable results without making model calls:

```sh
python3 scripts/check_fixtures.py
python3 docs/reports/m0-terra-replay-20260909/compile_report.py
```

Start a new full replay in a new output directory, using existing Codex
authentication and access to the explicitly requested model:

```sh
python3 docs/reports/m0-terra-replay-20260909/run.py \
  --output /tmp/repoctx-terra-new-replay
```

The runner refuses an existing output directory. A new run makes 12 new model
calls and requires fresh semantic grading; rerunning the report compiler uses
the retained run only. Model outputs need not reproduce exactly, even with the
same corpus and settings.

Primary artifacts: [results.json](results.json), [judgments.json](judgments.json),
[pilot executions](pilot/summary.json), and [batch executions](batch/summary.json).
The small synthetic corpus, single trial per task, full-source assistance,
manual grading, and unavailable immutable model snapshot limit generalization.
No claim of improved repoctx retrieval or real-world coding success follows.
