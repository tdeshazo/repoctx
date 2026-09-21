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
quality, or predeclare the experimental controls and decision rules assigned to
M4-03 through M4-05.

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
accounting, decision thresholds, or comparative performance. Those remain the
separate M4-03 through M4-05 gates.
