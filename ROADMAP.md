# repoctx roadmap

Updated 2026-09-22. This is the active development plan, not a delivery commitment.

Make repoctx reliably find useful, source-linked evidence within a task's budget.
Prioritize demonstrated retrieval failures and agent workflow results before
expanding the architecture.

## Current status

| Area | Status and evidence |
| --- | --- |
| Foundations, retrieval, input identity, and optional artifacts (M0–M3) | Recorded complete in the [historical plan](docs/history/roadmap-2026-09-22.md); preserve existing capabilities. |
| Evaluation (M4) | Fixtures, controls, accounting, and prototypes exist. M4-14 completed 33 paired tasks with 33/33 verified successes for ordinary tools and 32/33 with repoctx; it found no quality or efficiency benefit. Keep repoctx optional for small known-file tasks. See the [M4-14 practical report](docs/reports/m4-14-practical.md) and [frozen evaluation protocol](evals/m4/README.md). |
| Default workflow (M6-07) | Complete for the documented workflow checks described in the [completion report](docs/reports/m6-07-completion.md). The fresh bounded-read task passed; a final tuned replay of the same relationship task also passed after earlier partial runs. |
| Performance and distribution (M5–M6) | Recorded complete for their declared workloads and supported distribution. These results do not establish agent task success. |
| Canonical model (M7) | M7-01 through M7-03 implemented. Further expansion is paused; maintain the existing implementation. |
| Authority, semantic views, execution plans, and projections (M8–M11) | Deferred proposals, not the next delivery sequence. |

These statuses summarize retained evidence, not a fresh execution of its tests.
The [historical plan](docs/history/roadmap-2026-09-22.md) preserves milestone IDs,
completion links, and architectural proposals. This file owns current priorities.

## Immediate follow-up: real-repository skill evaluation

The 2026-09-22 external Tornado follow-up passed 8/8 tasks in every cohort.
The skill/no-prefill cohort used 28.5% more total input/output tokens than
vanilla (including cached input), with 11 command events failing on read ranges
beyond EOF. Historical controls and simultaneous treatment changes prevent
attributing the usage difference to the skill. Keep Repoctx optional; the
earlier M6-07 directed pass does not establish reliable EOF handling across tasks.
See the [actionable feedback](docs/reports/real-repoctx-skill-2026-09-22-feedback.md)
for evidence, limitations, and verification criteria.

- [ ] **First: fix recurring EOF read friction.** Inspect the evaluated version
  and failed commands, reproduce the errors, and make valid bounded retries
  obvious. Verify exact spans, output limits, and recovery on fresh agent tasks.
- [ ] **Next: reduce demonstrated retrieval overhead.** Inspect the largest
  regressions and apparent wins before changing guidance. Remove observed
  redundant work and retain the stop rule for sufficient or known-file evidence.
- [ ] **Before rerunning: repair external harness accounting.** Reconcile PATH
  and explicit-path invocations to 37 commands and 39 subcommands; distinguish
  the 11 Repoctx EOF failures from the one downstream public-check failure.
- [ ] **Then: run a controlled comparison.** Pin versions, isolate treatment
  changes, interleave repeated trials, and freeze fresh tasks and decision rules.
  Report correctness, usage components, time, and compilation cost separately.

## 1. Bounded retrieval corrections

M4-12 records the bounded retrieval corrections under realistic output limits.
The M4-13 paired workflow pilot and the M4-14 practical paired comparison below
are complete. M4-14 supports keeping repoctx optional for small known-file
tasks; no expansion or forced usage follows from this run.
The corrections address three [recorded context failures](docs/reports/m4-09-evidence.md):
identifier noise, an answer-bearing window that did not fit, and a clipped
reference to the actionable requirement.

- [x] **M4-12 — Reproduce and fix remaining bounded-retrieval failures.** Replay
  the three recorded cases against current code before choosing fixes. Distinguish
  already-resolved cases from remaining identifier matching, window selection,
  and budget failures; retain focused regression coverage for the decisive evidence.
  Evidence: [bounded retrieval replay](docs/reports/m4-12-bounded-retrieval.md).

Prefer exact identifier matches when appropriate without discarding useful
filename or prose evidence. Use query-centered windows and leave enough room for
evidence after response metadata. Preserve diverse relevant results without
allowing weak matches to displace decisive evidence. Use the existing discovery
and context paths; add a new abstraction only if a reproduced failure requires
it.

**Done when:** Small fixtures retrieve the decisive evidence at the recorded
5,000-, 8,000-, and 12,000-byte budgets, with exact source spans and explicit
excerpts or omissions. Check nearby queries, mixed filename/prose queries, and
no-answer cases for regressions.
Document any case that still cannot fit and its usable follow-up retrieval.
A local regression improvement does not establish downstream agent improvement.

## 2. Test the actual agent workflow

- [x] **M4-13 — Run a small paired workflow pilot.** Compare ordinary repository
  tools with the same tools plus repoctx. Include index-free discovery explicitly;
  use compilation when the task benefits from symbols and relationships. Evidence:
  [workflow pilot report](docs/reports/m4-13-workflow-pilot.md).

Choose a small, fixed set of representative documentation, code-localization,
cross-file, and software-change tasks before running either condition. Use the
same model, permissions, task budgets, and fresh task workspaces; interleave the
conditions. Keep expected answers outside agent inputs and check software changes
independently. Record task correctness, total time, raw model usage, all tool and
follow-up calls, failures, and any compilation cost. Reuse the existing evaluation
tooling where it fits; additional infrastructure is not the pilot's deliverable.

**Done when:** Paired results and failure traces support a concrete decision:
fix a recurring failure, simplify a feature that adds friction, or proceed to the
larger M4 study. An inconclusive or negative result is useful and must remain visible.

The completed synthetic pilot covered four tasks. Both conditions reached 4/4;
ordinary tools used 133.21 seconds, 15 tools, and 356,187 input+output tokens,
while the repoctx condition used 135.53 seconds, 16 tools, and 363,895
input+output tokens. The result did not establish an efficiency or coding-success
improvement; the completed M4-14 comparison below provides the larger practical
check without establishing a general efficiency or coding-success benefit.

This is exploratory development evidence. Keep its tasks separate from the M4
corpus if using the results to tune retrieval. The existing [frozen confirmatory
protocol and decision rules](evals/m4/README.md#predeclared-decisions) remain
optional background and unchanged; do not relabel this pilot or the practical
comparison as full-protocol compliance. Persistence and compaction experiments
remain separate and optional.

- [x] **M4-14 — Run the practical paired workflow comparison.** Completed all
  33 M4 corpus tasks as matched ordinary-tools and ordinary-tools-plus-repoctx
  pairs. Independent grading retained failures and costs; results are
  descriptive and make no statistical-generalization claim. See the [practical
  report](docs/reports/m4-14-practical.md) and its linked raw run and grading
  records. The decision is to keep repoctx optional for small known-file tasks;
  the existing 572-trial protocol remains unchanged optional background.

## 3. Simplify the default path

- [x] **M6-07 — Make discover → inspect → expand the obvious workflow.** Align
  CLI guidance, README examples, and the agent skill around finding useful
  evidence with minimal setup. Use pilot friction to guide any behavior changes.
  Evidence: [workflow verification report](docs/reports/m6-07-completion.md).

The discovery-first guidance is implemented in the README and agent skill. The
fresh bounded-read verification passed. The artifacts trace was partial in the
initial run and after a medium-effort skill replay. A final tuned replay of the
same task passed with revised skill guidance and `gpt-6-sol` at high effort; it
does not isolate the guidance effect. Earlier partial results remain visible in
their original reports.

The initial first-use run passed the documentation and discovery tasks in both
conditions, but both answers to the cross-file relationship task omitted the
required `build` → `choose` → `chooseSymbols` intermediate. The run also
observed multiword literal searches returning no results. A later fresh
verification passed EOF recovery, while its first artifacts answer confused
sibling calls in a headline; the final tuned replay passed all five frozen
claims. The README and agent skill explain the
literal-versus-task search choice, bounded EOF recovery, and caller/callee edge
verification. These results carry no efficiency or natural-adoption claim.

The focused treatment replay still omitted the required cross-file chain and
did not name the relationship source locations. It also guessed read endpoints
beyond EOF, producing bounded-read usage errors. Retain this observed case as a
regression reference; the completion report records the later directed checks
and their limits.

Start with index-free discovery and bounded reads. Introduce compilation when
indexed symbols and relationships help. Keep manifests, artifact catalogs,
persisted bundles, and checkpoints optional. An ordinary repository lookup must
not require authoring metadata or configuring a session store.

**Done when:** Representative first-use tasks reach useful evidence through the
documented workflow, with clear next reads when output is incomplete. Capture
setup and follow-up friction in the paired pilot rather than inferring usability
from command count alone. Documentation improvements can proceed alongside it.
M6-07 is complete for the documented workflow checks above; the confirmatory M4
evaluation remains a separate open gate.

## Requirements to preserve

- Exact source bytes, source references, and explicit incompleteness.
- Limits on the final serialized output; exact token caps require a tokenizer.
- Caller-owned scope, freshness verification, and bounded resource use.
- Repository declarations remain evidence and cannot authorize execution.
- Syntax and declared relationships remain distinguishable from verified behavior.
- Current contract identifiers and relevant conformance checks stay synchronized.

See the [context contract](docs/AGENT_CONTEXT.md), [discovery contract](docs/discovery.schema.json),
and [compatibility policy](docs/COMPATIBILITY.md) for details. These requirements
apply to shipped behavior without requiring every optional experiment to finish.

## Deferred directions

Pause M7-04/M7-05 and M8–M11 expansion: broader compilation identity, authority
profiles, semantic views, execution/verification plans, vendor projections, and
maintenance feedback. Retain implemented M7 capabilities and fix correctness
issues in them; pausing expansion does not waive existing input-identity or scope
requirements.

Embeddings, more grammars or semantic providers, a structural query language,
remote transport, a daemon, and additional persistence features also need a
specific observed workload gap before entering the active plan. Resume a proposal
when evidence identifies the user problem, the smallest useful change, and how
its benefit will be checked. Historical numbering does not determine priority.

## Working practice

Use **problem, intended behavior, verification** as the normal task format.
Keep changes small and independently reviewable. The PR and relevant tests are
normally sufficient completion evidence; separate reports are for benchmarks,
releases, or consequential design decisions. Record meaningful contract or trust
changes briefly, without duplicating requirements across documents.

Update this roadmap when priorities or completion change. Link durable evidence
where it helps, and keep raw runs and historical plans out of ordinary discovery.
For indexed dogfooding, apply equivalent caller exclusions because compilation
does not honor ignore files. Do not add report-size gates or a separate approval
step for routine work.

Progress means more tasks answered or changed correctly from bounded, traceable
evidence, with less setup and avoidable retrieval work.
