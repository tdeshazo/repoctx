# M4-14 practical comparison evidence

See the [report](../../../docs/reports/m4-14-practical.md) for results and the
development decision. This archive contains 66 trials: 33 tasks in two conditions.
It is a descriptive run, separate from the optional 572-trial protocol.

- `protocol.initial.json`, `prompts/`, `prepared/`, and `setup/` preserve the
  schedule, scoped inputs, executable identity, and permission checks.
- `trials/` preserves answers, raw events, source diffs, and per-trial records;
  `trials.jsonl` collects those records.
- `grading/summary.json` and `grading/operator-results.json` contain the final
  paired counts and per-trial outcomes. The [grading notes](grading/README.md)
  explain independent reviews, adjudications, change checks, and cost measures.
- `SHA256SUMS` hashes every other file in this archive.

The model was the available `gpt-6-sol` alias, at medium reasoning. The process
timeout was enforced; the 24-call limit was advisory. Operator scripts preserve
the implementation used, including temporary absolute paths; they are evidence,
not a new supported runner. No credentials, executables, or live workspaces are
included.
