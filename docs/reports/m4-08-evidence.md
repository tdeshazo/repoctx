# M4-08 persisted-evidence evaluation

Date: 2026-09-21

## Outcome

The frozen M4 protocol now hashes a paired persistence contract and evaluator.
It assigns all 30 answerable held-out tasks to matched inline/file-reference
recovery pairs and preassigns the three edge tasks to source mutation, missing
handle, and expired handle. This prevents post-outcome scenario selection.

Every trial attests that compaction removed prior evidence and that the decisive
detail is absent from its bounded checkpoint. The evaluator verifies matched
task, permission, budget, model, and compaction identities and requires a
complete record of initial delivery, reference/preview overhead, all subsequent
reads, repeated evidence, model usage, tool calls, end-to-end latency, and
time-integrated storage. It separately reports verified task success, recovery,
stale rejection, explicit missing/expired errors, and use after rejection.

Reports bind the protocol, task manifest, persistence contract, and canonical
records. Condition totals measure the whole task rather than treating a smaller
initial response as savings. The new predeclared
`persisted_file_reference_vs_inline` decision reuses M4-05's paired bootstrap,
5-point task-quality guard, and token/call/latency thresholds.

No held-out model outcomes were generated or inspected in this milestone.
Synthetic records only test the validator and reporter. Reports remain
`experimental_unassessed` until a locked 33-pair run and predeclared intervals
meet the M4-05 decision rules; no efficiency or task-quality benefit is claimed.

## Verification

```sh
python3 scripts/m4_persistence.py validate
python3 scripts/m4_protocol.py validate
python3 scripts/m4_decisions.py validate
python3 -m unittest tests.test_m4_persistence tests.test_m4_protocol \
  tests.test_m4_decisions
PYTHONPATH=src python3 -m unittest discover -s tests
go test ./...
go vet ./...
```

Regression cases cover incomplete pairs, control drift, task/compaction identity
drift, biased scenario reassignment, insufficient quality pairs, checkpoint
leakage, incomplete read traces, inline-storage misaccounting, recovery/read
overhead, expected lifecycle rejections, and reuse of M4-05 decision thresholds.

Dogfooding the advanced catalog found that an exact `M4-09` discovery query
returned an earlier planning mention while clipping the actionable roadmap item
from the selected window. The trace is assigned to M4-09's existing
under-retrieved/buried diagnosis scope rather than duplicated.
