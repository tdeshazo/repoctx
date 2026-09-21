# M4-04 complete accounting evidence

Date: 2026-09-21

## Outcome

The M4 run lock now pins the accounting contract and implementation alongside
the corpus, conditions, prompts, scoring, and binary identity. The accounting
validator requires exactly one record for every scheduled trial and records:

- compilation, update, retrieval, agent, and verification stage status and
  separate wall time, CPU time, peak memory, and byte measurements;
- an independent end-to-end trial wall time; stage and tool-call timings may be
  nested and are not summed into it;
- cold/warm requested state, supplier cache observations before and after, and
  the observed or unknown OS-cache state;
- every permitted tool call, including failures and timeouts, with its own
  duration and byte counts, plus a required complete-trace marker;
- completed, failed, timed-out, and abstained outcomes; and
- unchanged raw provider usage plus a small normalized token view.

Unavailable stage or provider measurements remain `null`, never zero. A report
adds only normalized input and output tokens. Cached-input tokens are reported
as a subset of input; reasoning tokens are reported as a subset of output.
Validation rejects either subset when it exceeds its parent.

The deterministic report includes complete trial records and compact summaries
for each track, condition, and cold/warm stratum. Stage summaries remain
separate. Tool calls and outcome counts are not hidden by successful aggregate
results. Output is refused inside the indexed repository so generated run data
cannot become source evidence.

## Verification

```sh
python3 scripts/m4_accounting.py validate
python3 -m unittest tests.test_m4_accounting tests.test_m4_protocol -v
PYTHONPATH=src python3 -m unittest discover -s tests -v
go test ./...
go vet ./...
```

The contract and reporter were tested with synthetic accounting records. The
held-out model study remains unrun until M4-05 predeclares aggregate decision
rules, so this evidence makes no retrieval-quality, cost, or outcome claim.
