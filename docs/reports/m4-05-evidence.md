# M4-05 predeclared decision rules

Date: 2026-09-21

## Outcome

The M4 run lock now hashes the decision rules and classifier before held-out
execution. The frozen primary metrics are verified task success for end-to-end
trials and required-span recall for evidence-only trials. Confirmatory results
use 95% paired task-cluster bootstrap intervals with 10,000 seeded resamples;
cache strata remain within task clusters.

Predeclared confidence-bound thresholds are:

| Claim | Quality guard | Practical threshold |
| --- | --- | --- |
| Repoctx graph vs ordinary tools | task success no worse than -5 points | 15% token, 20% tool-call, or 15% wall-time reduction |
| Repoctx without graph vs lexical windows | required-span recall no worse than -5 points | 15% selected-evidence-byte reduction |
| Graph vs no graph | none | 3-point required-span-recall improvement |

Efficiency claims are metric-specific. The overall efficiency result is
supported only when the quality guard passes, at least one efficiency metric is
supported, and no efficiency interval remains inconclusive. A quality interval
wholly below its tolerance is reported as harm even when costs fall.

Confirmatory classification requires at least 30 paired tasks and no more than
5% missing pairs. Failures and timeouts count as task failures and retain costs;
abstentions are scored against the declared outcome. A crossing interval is
inconclusive. An interval wholly below a practical threshold is
`threshold_not_met`, not proof of equivalence.

Artifact-aware and human-oracle comparisons are descriptive only. Their
attribution remains repository declarations plus harness selection and human
gold, respectively; neither can become a repoctx claim.

Every classifier input and output retains the SHA-256 identities of the frozen
run lock, accounting report, and scored results so intervals cannot be published
without naming the exact evidence products they summarize.

## Verification

```sh
python3 scripts/m4_decisions.py validate
python3 -m unittest tests.test_m4_decisions tests.test_m4_protocol -v
PYTHONPATH=src python3 -m unittest discover -s tests -v
go test ./...
go vet ./...
```

Synthetic intervals exercise support, harm, threshold-not-met, missing, and
underpowered paths. No held-out outcomes were inspected and no quality or
efficiency claim is made by this milestone.
