# M4-14: practical paired workflow comparison

Status: complete. This descriptive engineering comparison ran all 33 M4 tasks
as matched fresh, scoped pairs: ordinary repository tools versus ordinary tools
plus repoctx. The available `gpt-6-sol` alias ran at medium reasoning with the
20260922 seeded schedule, a 300-second timeout, and a 24-call advisory limit.
The alias was not pinned to an immutable model revision.

| Condition | Verified success | Median agent wall time | Input + output tokens | Shell calls | Observed tool items |
| --- | ---: | ---: | ---: | ---: | ---: |
| Ordinary tools | 33/33 | 18.76 s | 2,470,076 | 104 | 111 |
| Ordinary tools + repoctx | 32/33 | 22.58 s | 2,828,347 | 142 | 149 |

Agent wall time is Codex process time for the trial. It excludes repository
export and build preparation, grading, and independent change-check time. Usage
was present for every trial; there were no timeouts or tool-cap overages.

Independent subagent grading retained failures and edge cases. All three edge
cases passed in both conditions, and all 12 independent change checks passed.
The sole treatment quality difference was one partial accessibility answer with
weak grounding; its baseline pair passed. Both conditions edited the same
ambiguous non-change question, which is retained as an observation rather than
treated as a product requirement.

Actual treatment adoption is reported separately from assignment: 19/33 trials
read the repoctx skill, 14/33 invoked `repoctx version` (two were version-only),
and 12/33 invoked repoctx discovery/retrieval with output observed. No indexed
commands or additional repoctx follow-up requests were invoked. The comparison
therefore does not treat assignment to the treatment condition as proof of
repoctx use.

The result shows no proven quality or efficiency benefit on this small synthetic
corpus. Ordinary tools are appropriate for small known-file tasks, so the
decision is to keep repoctx optional and retain the current minimal workflow.
No expansion or forced usage follows from this run, and the result makes no
statistical-generalization claim. The existing 572-trial confirmatory protocol
remains unchanged optional background; this report is not a full-protocol claim.

Evidence: [run README](../../evals/runs/m4-14-practical-20260922/README.md),
[raw run](../../evals/runs/m4-14-practical-20260922/), [grading
summary](../../evals/runs/m4-14-practical-20260922/grading/summary.json),
[operator results](../../evals/runs/m4-14-practical-20260922/grading/operator-results.json),
[adoption audit](../../evals/runs/m4-14-practical-20260922/grading/adoption-audit.json),
and [per-review grades](../../evals/runs/m4-14-practical-20260922/grading/grades/).
