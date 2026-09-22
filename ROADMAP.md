# repoctx roadmap

Updated 2026-09-22. This is the active development plan, not a delivery commitment.

Make repoctx reliably find useful, source-linked evidence within a task's budget.
Prioritize demonstrated retrieval failures and agent workflow results before
expanding the architecture.

## Current status

| Area | Status and evidence |
| --- | --- |
| Foundations, retrieval, input identity, and optional artifacts (M0–M3) | Recorded complete in the [historical plan](docs/history/roadmap-2026-09-22.md); preserve existing capabilities. |
| Evaluation (M4) | Fixtures, controls, accounting, and prototypes exist. Agent-outcome acceptance gates remain open; no demonstrated agent-efficiency or coding-success improvement. See the [evaluation protocol](evals/m4/README.md). |
| Performance and distribution (M5–M6) | Recorded complete for their declared workloads and supported distribution. These results do not establish agent task success. |
| Canonical model (M7) | M7-01 through M7-03 implemented. Further expansion is paused; maintain the existing implementation. |
| Authority, semantic views, execution plans, and projections (M8–M11) | Deferred proposals, not the next delivery sequence. |

These statuses summarize retained evidence, not a fresh execution of its tests.
The [historical plan](docs/history/roadmap-2026-09-22.md) preserves milestone IDs,
completion links, and architectural proposals. This file owns current priorities.

## 1. Bounded retrieval corrections

M4-12 records the bounded retrieval corrections under realistic output limits.
After these corrections, the next outstanding roadmap item is M4-13, the paired
actual agent workflow pilot below. Complete that pilot before further retrieval
tuning or infrastructure work.
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

- [ ] **M4-13 — Run a small paired workflow pilot.** Compare ordinary repository
  tools with the same tools plus repoctx. Include index-free discovery explicitly;
  use compilation when the task benefits from symbols and relationships.

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

This is exploratory development evidence. Keep its tasks separate from the M4
held-out set if using the results to tune retrieval. Preserve the existing
[confirmatory protocol and decision rules](evals/m4/README.md#predeclared-decisions)
for published improvement claims; do not relax them or label the pilot as M4
completion. Persistence and compaction experiments remain separate and optional.

## 3. Simplify the default path

- [ ] **M6-07 — Make discover → inspect → expand the obvious workflow.** Align
  CLI guidance, README examples, and the agent skill around finding useful
  evidence with minimal setup. Use pilot friction to guide any behavior changes.

Start with index-free discovery and bounded reads. Introduce compilation when
indexed symbols and relationships help. Keep manifests, artifact catalogs,
persisted bundles, and checkpoints optional. An ordinary repository lookup must
not require authoring metadata or configuring a session store.

**Done when:** Representative first-use tasks reach useful evidence through the
documented workflow, with clear next reads when output is incomplete. Capture
setup and follow-up friction in the paired pilot rather than inferring usability
from command count alone. Documentation improvements can proceed alongside it.

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
