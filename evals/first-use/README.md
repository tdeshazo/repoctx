# M6-07 first-use tasks

This is a fixed, three-task operator corpus for realistic first-use validation
of the documented discovery-first workflow. It uses the repository source at
`repoctx-source@1dc9657c30c06ff26320fead0a23d28fbd947781`, not a synthetic
fixture. It is not a held-out evaluation set and supports no aggregate,
comparative, or generalization claim.

For each trial, give the agent one file from `agent_inputs/` and a fresh source
export at that revision. The export excludes `evals/`, `scripts/`,
`docs/reports/`, `docs/history/`, `.agents/`, `.codex/`, `.git`, `build/`, and
temporary or generated build artifacts. It retains `README.md`, `ROADMAP.md`,
source, contracts, and the vendored `skills/repoctx/` guidance. The input prompt
is the complete task; source is the only task evidence.

`evaluator/expected.json` is operator-only. It records the expected answer
claims, exact source anchors, and task-local scoring. Do not provide it to an
agent. Before an execution, verify the frozen inputs and operator key:

```sh
sha256sum -c evaluator/freeze.sha256
```

## Reproduce the recorded paired run

The runner exports source from `1dc9657`; it does not use the current checkout's
README or skill as trial evidence. It requires a checkout that still contains
that commit, a `repoctx` binary built from that revision whose `version -format
json` reports the same revision, and an authenticated Codex CLI that supports
the named `pilot` profile used by the runner. The run creates six live model
trials and consumes model usage.

From a checkout containing the pinned commit, build the binary from a separate
pinned worktree and give the runner a new directory outside the repository:

```sh
git worktree add /tmp/repoctx-first-use-source 1dc9657
(cd /tmp/repoctx-first-use-source && go build -o /tmp/repoctx-first-use)
python3 scripts/run_first_use.py --repoctx /tmp/repoctx-first-use \
  --run-dir /tmp/repoctx-m6-07-new
```

The runner verifies the frozen inputs and key, source revision, binary revision,
and the permission profile before the six trials. Its recorded protocol treats
model aliases as unresolved and does not control provider cache behavior, so a
new run is a new observation. Setup, operator work, and local build cost are
not included in trial records.

The original run record and approved raw archive are in
[`docs/reports/m6-07-first-use.md`](../../docs/reports/m6-07-first-use.md) and
[`evals/runs/m6-07-20260922`](../runs/m6-07-20260922). The separate targeted
guidance replay, including its exact README and skill overlay snapshots, is in
[`evals/runs/m6-07-relationship-replay-20260922`](../runs/m6-07-relationship-replay-20260922).

The tasks cover a direct documentation answer, the intended discovery-to-
focused-read follow-up, and an indexed cross-file relationship question.
Record the observed workflow and response completeness. A manual deterministic
small-output walkthrough, if needed, is separate from paired-agent evidence.
Compilation remains a natural choice for the third task, not a required agent
action.
