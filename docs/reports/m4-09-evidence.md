# M4-09 context-failure diagnosis

M4-09 adds a deterministic, trace-backed diagnosis contract rather than a new
retrieval heuristic. [`failures.json`](../../evals/m4/failures.json) defines four
non-exclusive tags and controlled contributor vocabularies.
[`failure-cases.json`](../../evals/m4/failure-cases.json) records the three
required dogfooding incidents with replay arguments, source revisions,
evidence-marker hashes, omission signals, and compact observations.

## Ranked findings

| Priority | Incident | Diagnosis | Principal contributors |
| ---: | --- | --- | --- |
| 133 | M4-07 8 KiB no-result trace | under-retrieved, buried | selection window size; output budget |
| 132 | M4-06 `M4-07` identifier-noise trace | over-retrieved, buried | generated-input inventory; tokenization/ranking; output and result budgets |
| 123 | M4-08 clipped actionable item | under-retrieved, buried | window boundary; output budget |

The highest-ranked failure is selection/budget-bound, while the next is spread
across inventory, discovery, and budget behavior. That distinction is the main
result: neither failure supports assuming that ranking alone is the appropriate
fix. All three records carry multiple derived tags, and the report also groups
the named contributors for remediation planning.

## Reproduction

```sh
python3 scripts/m4_failures.py validate
python3 scripts/m4_failures.py report --output /tmp/m4-failure-report.json
python3 scripts/m4_protocol.py validate
```

The M4 run lock hashes the diagnosis contract, cases, and validator with the
other frozen evaluator inputs. Reports are compact JSON outside the indexed
repository so diagnostic output does not become new retrieval evidence. These
historical cases validate classification and prioritization behavior; they do
not establish downstream quality improvement.
