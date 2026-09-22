# Real repository skill comparison

This protocol prepares a future controlled comparison of Repoctx skill guidance
on fresh repository tasks. It does not rerun or reinterpret the eight-task
September report. The primary comparison isolates whether showing the pinned
native skill helps when both conditions have the same pinned Repoctx executable.
The existing [M4 protocol](../m4/README.md) remains the source for the project's
paired confidence and decision conventions.

## Primary contrast

| Control | Treatment |
| --- | --- |
| Same Repoctx executable available on `PATH`; skill hidden; no prefilled evidence | Same executable; exact pinned skill visible through normal native skill discovery; no prefilled evidence |
| Empty private Repoctx index at trial start | Empty private Repoctx index at trial start |
| Same task, model, prompt, permissions, budgets, workspace revision, and tools | Same task, model, prompt, permissions, budgets, workspace revision, and tools |

The user task prompt must not name Repoctx or contain retrieval advice. In the
control, the executable remains discoverable through the same `PATH`; the
assigned difference is native skill visibility. This estimates the incremental
effect of the skill package in that environment. It does not estimate the total
effect of adding Repoctx to an environment without the tool.

Draft a new task set after the skill and binary revisions are fixed. Do not tune
against the report's eight tasks or reuse them as confirmatory evidence. Freeze
each task prompt, repository revision and exported tree, hidden grader, and any
independent change check before the first model trial. Keep evaluator answers
outside model-readable workspaces. Use at least 30 distinct tasks and three
attempts per task and condition. The task set must include at least five
known-file tasks, ten navigation or relationship tasks, and five change tasks; the
remaining tasks may add to those families. Preserve known-file controls and
include tasks with room for a quality improvement.

## Freeze and schedule the run

The operator supplies two JSON files outside the model workspace. The task
manifest contains identity and digest metadata only, never task text or answers:

```json
{
  "version": "repoctx.real-skill-comparison.tasks/v1",
  "tasks": [
    {
      "id": "fresh-nav-001",
      "family": "navigation_or_relationship",
      "repository_revision": "<full 40- or 64-character Git revision>",
      "repository_tree_sha256": "<64 lowercase hex characters>",
      "task_sha256": "<64 lowercase hex characters>",
      "grader_sha256": "<64 lowercase hex characters>"
    }
  ]
}
```

Allowed family values are `known_file`, `navigation_or_relationship`, and
`change`. Use a unique task ID for each independent question. When prompts or
graders are files, hash their exact bytes. The repository tree digest covers the
export the runner will provide; it is separate from the immutable Git revision.

The run lock must use version `repoctx.real-skill-comparison.lock/v1` and pin
these fields:

| Field | Required identity |
| --- | --- |
| `model` | Provider, immutable model ID and revision, and reasoning effort |
| `repoctx` | Full source revision, executable SHA-256, and SHA-256 of `repoctx version -format json` output |
| `skill` | Full source revision and SHA-256 of a canonical file-path/digest inventory for the complete skill tree |
| `harness` | Clean project revision, this schedule generator's SHA-256, and exact client name and version |
| `execution` | SHA-256 of the shared permission profile, SHA-256 of the shared tool profile, and one common budget object |

The budget object has positive integer values for `max_context_tokens`,
`max_output_tokens`, `max_tool_calls`, `max_trial_seconds`,
`max_tool_output_bytes`, and `tool_timeout_seconds`. The same object and
permission/tool profile apply to both conditions. The harness pins `reasoning_effort`
as part of model identity. Do not start a confirmatory schedule with a mutable
model alias, dirty harness checkout, missing digest, or unpinned skill/binary.

On a clean, committed checkout, freeze the randomized schedule:

```sh
python3 evals/real-skill-comparison/prepare_schedule.py \
  --tasks /secure/evaluator/tasks.json \
  --lock /secure/evaluator/run-lock.json \
  --output /secure/evaluator/frozen-schedule.json
```

The generator checks the minimum task count and family mix; full Git revisions
and SHA-256 values; pinned, well-formed model, binary, skill, client, and harness
identity declarations; the current clean project revision; the generator
digest; and positive shared budgets. It creates three matched repeats per task,
places the two conditions
for each task/repeat adjacent in randomized order, balances which condition goes
first, and shuffles pair order with the seed in `protocol.json`. It refuses to
overwrite a schedule. Archive the task manifest, run lock, protocol, generated
schedule, and schedule digest together before execution.

The schedule freezer prepares assignments; it does not run an agent or enforce
runtime isolation. The runner must verify the pinned identities again, export a
fresh writable workspace for every trial, apply the same permissions and
budgets, keep graders inaccessible, and record all scheduled attempts. If the
runtime fails to complete a pair, retain the partial attempt and its costs.

## Costs and cache observations

Start every primary trial with empty Repoctx scratch and no selected evidence
provided to the agent. If the agent chooses to compile an index, count that
time, bytes, failure, and follow-up work in the trial. Record Repoctx index state
before and after, observed OS cache state (or `unknown`), provider cached-input
tokens, uncached input, output, reasoning tokens, tool calls and failures, and
end-to-end trial time. Use null when the provider does not expose a usage field;
never turn missing data into zero.

Build the pinned executable before trials and report build time and resource use
as setup. If studying prefilled evidence or a prebuilt index, hold skill
visibility fixed and run a separate comparison that changes only that factor.
Record evidence preparation, index compilation, and index size, then show task
cost with setup excluded and all-in cost at each declared deployment volume.
Do not pool these comparisons with the primary skill contrast.

## Scoring and decision

Blind graders to condition. Score every attempt with frozen answer criteria and
run independent change checks on fresh source copies. Keep failures, timeouts,
and abstentions in the assigned condition and retain their measured costs.
Publish per-task results, all repeats, missing pairs, usage components, compile
and setup costs, traces, and hashes.

Use verified task success as the primary quality metric. Keep the
[M4-05](../../docs/reports/m4-05-evidence.md) quality guard of no more than a
five percentage-point reduction. Count skill-injection tokens as treatment
input. Report tokens, tool calls, and trial wall time separately, with the
M4-05 practical thresholds of
15% fewer input-plus-output tokens, 20% fewer tool calls, or 15% less trial wall
time. Use a 95% paired task-cluster bootstrap with 10,000 fixed-seed resamples;
keep all three repeats within their task cluster. Apply the project's M4-05
confidence-bound decision rules. A missing-pair rate above 5%, a failed quality
guard, or intervals that cross the declared thresholds do not support a broad
benefit claim. Publish an inconclusive result as inconclusive.

This protocol and schedule are preparation artifacts; no real-task trials have
been run under them. A completed execution and frozen analysis are still needed
before any effect claim.
