# Matched gpt-6-sol Repoctx comparison: actionable feedback

Recorded 2026-09-22 from the external report at
`~/Workspace/evals/reports/real-sol-vanilla-vs-repoctx-skill-2026-09-22-v1/REPORT.md`.
This review uses that report and its saved summary and command list; it does not
independently regrade the trials. The source files require the local evaluation
workspace.

## What the comparison establishes

The two arms each passed all eight frozen Tornado tasks, with one fresh attempt
per arm and task. The Repoctx arm used 5,402,521 input-plus-output tokens versus
5,274,784 for vanilla: 127,737 more, or **2.4%**. Its uncached input was
543,922 versus 480,403 (**13.2% more**), while output was 62,183 versus 64,141
(**3.1% less**). The median paired task token ratio was 1.103×; only two of
eight treated tasks used fewer tokens. Input totals include cached input, and
these measures are neither API prices nor subscription credits.

The skill was read and Repoctx was used in every treated task, as instructed.
There were 13 completed Repoctx command events: 12 `discover`, one `read`, and
no nonzero exits. The harness also built cold indexes for the treatment arm,
using 14.7 seconds in total outside agent token accounting. No indexed
relationship command appears in the reported Repoctx calls. That observation
does not establish whether the index had an indirect effect elsewhere.

The arms shared the task source, grader, model, reasoning, CLI, and time limit;
order alternated by task. The assigned treatment still bundled the skill, the
Repoctx executable and index, and an instruction requiring skill reading and at
least one targeted Repoctx call. The result therefore estimates that bundle
under forced use. It cannot isolate skill guidance, spontaneous use, index
value, or a benefit on tasks where the control already succeeds. This eight-pair,
single-repository result does not satisfy the separate
[fresh-task comparison protocol](../../evals/real-skill-comparison/README.md),
which calls for a common executable in both arms and repeated trials.

## Actions in priority order

1. **Keep use optional and maintain the quality bar.** Do not make Repoctx a
   required step for known-file or otherwise settled tasks based on 8/8 versus
   8/8 success and a small aggregate token increase. Verify the exact evaluated
   skill and binary revisions before treating these runs as a check of recent
   EOF or guidance changes. Zero failed commands here does not test EOF
   recovery: only one Repoctx `read` occurred.

2. **Inspect the large task-level differences before changing retrieval.**
   Review saved treatment and vanilla traces for `real-context-02` (1.619×) and
   `real-local-01` (1.355×), then the two lower-token cases,
   `real-workflow-01` (0.741×) and `real-local-02` (0.584×). For each, record
   phase, selected evidence, bytes returned, subsequent shell reads, skill
   exposure, retries, and whether the answer or change relied on Repoctx.
   State which extra work was necessary, repeated, or incidental before making
   a product change. Do not infer a general task-family effect from one attempt.

3. **Account for discovery and preparation separately.** Report the 12
   discovery calls and one read with returned bytes and follow-up work. Keep
   the 14.7 seconds of cold indexing visible as setup cost; the command mix
   gives no observed indexed relationship use to justify making compilation a
   default step. In the next run, measure task wall time and index preparation
   both separately and in an all-in cost at the intended use volume.

4. **Test the specific product decision on fresh tasks.** Use the
   [frozen comparison design](../../evals/real-skill-comparison/README.md):
   make the same pinned executable available in both arms, vary only native
   skill visibility, remove the forced-use instruction, match budgets and
   workspace state, and repeat each task. Freeze harder tasks and graders with
   room for quality improvement before running. Record cached and uncached
   input, output, correctness, calls, failures, and wall time; apply the
   predeclared paired decision rules. Study prebuilt indexes or explicit-use
   instructions as separate factors if those deployment choices matter.

This matched run is useful evidence that forced Repoctx use did not improve
observed success on these saturated tasks. It gives no basis to claim either a
general efficiency gain or a general efficiency loss.
