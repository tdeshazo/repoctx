# M4-01 held-out pilot task set

Date: 2026-09-21.

The [M4 pilot](../../evals/m4/README.md) now provides 30 answerable tasks split
evenly across three permission-cleared synthetic repositories, plus distinct
no-answer, contradictory-evidence, and denied-scope cases. Its answerable mix is
nine documentation lookups, nine code localizations, six cross-file analyses,
and six changes with evaluator-owned executable checks. The existing M0 corpus
is named only as separate tuning data; task-ID overlap is rejected.

The [manifest](../../evals/m4/manifest.json) maintains semantic source anchors
rather than derived hashes or offsets. The [validator](../../scripts/check_m4_tasks.py)
resolves exact evidence, validates scope and permission metadata, exports only
agent-visible inputs, and runs change checks. Regression tests export all 33
tasks, confirm denied/evaluator data is absent, prove source edits change derived
provenance, reject malformed permission and scope records, and show all six
change checks fail on baseline then pass after the requested edit.

This is a public pilot corpus, not a completed comparative experiment or a claim
of generalization. Evaluation conditions, controls, accounting, and decision
rules remain M4-02 through M4-05.

Verification:

- `python3 scripts/check_m4_tasks.py`
- `python3 -m unittest tests.test_m4_tasks -v`
- `python3 -m unittest discover -s tests -p 'test_*.py'`
- `go test ./...`
- `git diff --check`
