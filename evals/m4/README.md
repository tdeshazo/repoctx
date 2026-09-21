# M4 held-out pilot tasks

This public pilot contains 30 answerable tasks across three synthetic
repositories, plus one no-answer, one contradictory-evidence, and one
denied-scope case. Each repository was created for repoctx and is covered by the
project's Apache-2.0 license; its `PROVENANCE.json` records that origin and
redistribution status. The corpus is a pilot floor, not evidence of statistical
generalization or improved agent outcomes.

The 30 answerable tasks comprise nine documentation lookups, nine code
localizations, six cross-file analyses, and six changes. Every repository has
ten answerable tasks. M0 remains the explicitly separate tuning corpus; the
validator rejects task-ID overlap between the two sets.

## Contract and separation

[`manifest.json`](manifest.json) is evaluator-only. It declares questions,
expected outcomes, named source anchors, scope, and independent change checks.
Anchors contain semantic boundary text, not hashes or byte coordinates. The
validator requires unique anchors and derives exact SHA-256 and half-open byte
spans from repository bytes on every run.

An exported trial contains only `input.json` and the permitted repository
files. Expected answers, evidence selectors, outcome labels, check expectations,
and denied files remain outside the export. Because the held-out definitions are
committed publicly, harnesses must deny the evaluator tree to the agent; “held
out” means excluded from tuning, not confidential. Run each trial in a fresh
output directory.

Validate the corpus or export one task:

```sh
python3 scripts/check_m4_tasks.py
trial=$(mktemp -d)/run
python3 scripts/check_m4_tasks.py \
  --fixture l-doc-default-currency --output "$trial"
```

For a change task, run its evaluator-owned check against the modified export:

```sh
python3 scripts/check_m4_tasks.py \
  --verify-change l-change-late-cap --repository "$trial/repository"
```

Change checks import and execute a named function from these synthetic fixtures.
They are not a general repository-command runner and must not be redirected to
untrusted repositories. Each check fails on its baseline and passes after the
requested edit in the regression suite.

The corpus validator checks permission metadata, path confinement, symlinks,
UTF-8, unique evidence, scope, agent/evaluator separation, task floors, tuning
separation, and check ownership. It does not execute an agent, measure retrieval
quality, or establish comparative performance.

## Comparison conditions

[`conditions.json`](conditions.json) defines six explicitly attributed inputs:
ordinary filesystem tools without preselected evidence, independent bounded
lexical windows, repoctx context at graph depths zero and one, an optional
declared-artifact condition, and a human-curated evidence oracle. The latter two
are separate conditions: repository declarations plus harness selection are not
reported as autonomous repoctx retrieval, and human gold spans are never
reported as repoctx output. A missing repository catalog makes the
artifact-aware condition unavailable rather than silently substituting another
supplier.

Every evidence condition uses the same bounded output envelope. The lexical
selector is implemented independently in Python. Repoctx conditions use an
exported repository and require an explicit checkout-built binary. The oracle
contains only permitted source spans, not expected answers. Validate the matrix
or render one exported input with:

```sh
python3 scripts/m4_conditions.py validate
python3 scripts/m4_conditions.py render \
  --task l-cross-quote --condition bounded_lexical \
  --repository "$trial/repository"
```

Use `--repoctx ./repoctx` for either repoctx condition. Rendering prepares a
condition input; it does not run an agent or establish matched controls, cost
accounting, decision thresholds, or comparative performance. Accounting and
decision classification are separate steps below.

## Frozen experiment controls

[`protocol.json`](protocol.json) pins the compiler, context, model, permission,
budget, ordering, workspace, and cache-observation profiles.
[`scoring.json`](scoring.json) fixes per-trial rubrics without choosing the
aggregate decision thresholds reserved for M4-05. End-to-end conditions all
receive the same isolated repository shell. Evidence-only conditions run in a
separate track with no tools or runtime repository access; the human oracle is
available only in that track.

The seeded global schedule assigns a unique fresh export to every trial,
including every change trial, and interleaves conditions, tasks, tracks, and
cold/warm strata. “Cold” and “warm” describe harness-owned supplier state. Each
trial must record the requested state, supplier cache state before and after,
and the uncontrolled observed OS-cache state.

Validate the controls, then create an immutable run lock from a clean checkout:

```sh
python3 scripts/m4_protocol.py validate
go build -o /tmp/repoctx-m4 ./cmd/repoctx
python3 scripts/m4_protocol.py lock \
  --model-id MODEL_ID --model-revision IMMUTABLE_MODEL_REVISION \
  --repoctx /tmp/repoctx-m4 --output /tmp/m4-run-lock.json
```

The lock hashes the source inputs, prompts, scoring rules, protocol, harness,
and repoctx binary; records the full Git commit and repoctx build metadata; and
embeds the complete schedule. It refuses a dirty worktree, placeholder model
identity, incompatible binary, overwrite, or output inside the indexed
repository. Creating a lock does not execute trials; the reporter below consumes
their accounting records, and the decision classifier applies the frozen rules.

## Complete accounting

[`accounting.json`](accounting.json) defines five non-overlapping cost stages:
compilation, update, retrieval, agent, and verification. Each trial record also
contains end-to-end wall time, its observed cache state, outcome, every permitted
tool call, and both raw and normalized provider usage. Stage and tool-call times
may be nested and are never added to end-to-end latency. Missing measurements
remain `null`; failed calls and stages, timeouts, and abstentions remain explicit
records. Every trial must affirm that its tool trace is complete.

Generate a complete report only after every locked trial has one JSONL record:

```sh
python3 scripts/m4_accounting.py validate
python3 scripts/m4_accounting.py report \
  --run-lock /tmp/m4-run-lock.json --records /tmp/m4-records.jsonl \
  --output /tmp/m4-accounting.json
```

The validator rejects missing or duplicate trials, cache labels inconsistent
with the schedule, evidence-only tool calls, malformed stage measurements, and
subset token counts larger than their parents. Reports retain raw provider
usage and group cold/warm results by track and condition. The normalized total
adds only input and output tokens; cached input remains a subset of input, and
reasoning remains a subset of output. Reports are compact JSON written outside
the indexed repository. They provide accounting only; the classifier below
handles decision thresholds.

## Predeclared decisions

[`decisions.json`](decisions.json) freezes primary metrics, quality tolerances,
practical thresholds, sample requirements, and uncertainty reporting before any
held-out trial is run or inspected. Confirmatory intervals use a seeded 10,000-
resample paired task-cluster bootstrap at 95% confidence, stratified by track and
cache state. Failures and timeouts score as task failures, while abstentions are
scored against the expected outcome rather than dropped.

End-to-end efficiency requires the lower confidence bound for verified task
success to remain within 5 percentage points of ordinary tools. Metric-specific
claims then require at least 15% fewer input-plus-output tokens, 20% fewer tool
calls, or 15% lower trial wall time. The evidence-only selection comparison uses
the same 5-point recall tolerance and requires 15% fewer selected evidence
bytes. Graph expansion requires at least a 3-point recall improvement. These
thresholds apply to confidence bounds, not point estimates.

At least 30 paired tasks and no more than 5% missing pairs are required for a
confirmatory classification. Intervals crossing a guard or threshold remain
inconclusive; an underpowered result is never called equivalent or a
non-regression. Artifact-aware and human-oracle comparisons are descriptive
only and retain their repository/human attribution.

Validate the frozen rules or classify precomputed intervals:

```sh
python3 scripts/m4_decisions.py validate
python3 scripts/m4_decisions.py assess --input /tmp/m4-decision-input.json
```

The classifier verifies that the input names the exact predeclared comparison,
metrics, sample counts, missingness, bootstrap procedure, and SHA-256 identities
of the run lock, accounting report, and scored results. It does not calculate
intervals or turn descriptive comparisons into autonomous repoctx claims.
