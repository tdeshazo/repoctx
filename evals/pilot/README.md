# M4-13 exploratory pilot corpus

This fixed, synthetic corpus is a small feasibility fixture for paired trials.
It is not part of the M4 held-out corpus and does not support a performance or
generalization claim. The sole fixture, `repositories/relay-board`, was written
from scratch for this pilot and is covered by this project's Apache-2.0 license;
see its `PROVENANCE.json`.

## Trial boundary

For each fresh agent workspace, copy only the selected file from `agent_inputs/`
as `input.json` and the contents of `repositories/relay-board/` as
`repository/`. Do not give the agent this README's parent corpus directory,
`evaluator/`, or any other task input. The task input is the complete prompt;
the repository is the only evidence source.

`evaluator/expected.json` contains answer keys and evidence anchors. It is for
the trial operator only. The executable change check is likewise evaluator-only:

```sh
python3 evaluator/check_change.py --repository /path/to/modified/repository
```

The check deliberately fails against the unmodified fixture and succeeds after
the requested change. It uses only the Python standard library.

## Frozen task set

The four task IDs are fixed for this pilot: `p-doc-retry-delay`,
`p-code-queue-selection`, `p-cross-dispatch-flow`, and `p-change-batch-cap`.
Do not alter their prompts, repository bytes, expected records, or check before
the paired trials have finished. The evaluator records were derived from the
repository files in this directory on 2026-09-22.

Before a trial, the operator can verify the frozen corpus with:

```sh
sha256sum -c evaluator/freeze.sha256
```

## Run and inspect

Build the current CLI and use an authenticated Codex CLI with support for named
permission profiles (the recorded run used 0.155.1):

```sh
go build -o /tmp/repoctx-pilot-binary .
python3 scripts/run_workflow_pilot.py --repoctx /tmp/repoctx-pilot-binary \
  --run-dir /tmp/repoctx-pilot-new-run
```

Run these commands from the project root. The output directory must be new and
outside the project. This starts eight actual model trials and consumes model
usage. The runner uses a private temporary authentication directory, removes it
on completion, and restricts trial access to its workspace and runtime tools.
Before using a different host or CLI version, verify the named profile denies
reads of evaluator and sibling workspace files and denies network access.

See the [pilot report](../../docs/reports/m4-13-workflow-pilot.md) and
[raw archive](../runs/m4-13-20260922/valid/protocol.json). The archive preserves
original records, including the old counter's omission of file edits; use
`scored-results.json` for audited counts. `runner-source.txt` is the exact measured
runner (its SHA matches the protocol), while the current script includes the
post-pilot primer and accounting corrections. To replay that implementation,
restore the snapshot under `scripts/` so its project-root lookup remains valid.
Model alias, provider cache, and host differences prevent exact result replay.
