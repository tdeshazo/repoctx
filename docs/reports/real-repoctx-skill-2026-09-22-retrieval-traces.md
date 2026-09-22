# Retrieval overhead trace notes

Read-only review of the saved `real-repoctx-skill-v1` events under
`/home/travis/Workspace/evals/runs` and per-task rows in the v2 report. These
observations describe tool use; the historical, single-run controls do not
support attributing token differences to the skill.

| Task | Skill / vanilla total | Trace finding |
| --- | ---: | --- |
| `real-context-01` | 1.938× | One 18 KB discovery returned 23 excerpts. A five-file read succeeded (~36.9 KB output); two later batched reads failed on guessed ends (`template.py:1000:1230` exceeded 1045 lines; `template_test.py:550:830` exceeded 528), then a corrected six-file read succeeded (~35.7 KB). The failed diagnostics were small, but each prompted another tool call. Further shell reads inspected implementation and test setup. |
| `real-local-02` | 1.716× | One 18 KB discovery returned 25 excerpts, including matches in unrelated CLI modules and repeated nearby test lines. There were no failed RepoCtx calls. Follow-up shell output included `options.py:125-430` and a later `options.py:400-485`, repeating lines 400-430. |
| `real-workflow-02` | 0.932× | Three separate stages each read the skill and discovered their component; each stage had one failed batched read. Client and wire stages corrected the bad range with `:0`; the handler stage switched to shell reads. The separate stage contexts explain the three skill reads. |
| `real-navigate-01` | 0.900× | A cross-layer task used one broad discovery, two exact-symbol searches, and two multi-file reads. One read failed on an end beyond EOF and was retried. The exact searches found definitions not present in the discovery result list, so the trace does not establish that they were redundant. |

The repeated failed range requests are directly supported as avoidable tool
calls. The `real-local-02` trace also shows a small overlapping shell read, but
most of its source inspection was not demonstrably redundant. The two lower
token totals belong to distinct task setups and are not evidence that any
particular retrieval pattern saved tokens.

The focused guidance update in [the RepoCtx skill](../../skills/repoctx/SKILL.md)
asks agents to correct all invalid ranges reported by a batched read before
retrying and to request only lines missing from existing evidence. Evaluate
whether this reduces failed calls or repeated output on fresh matched tasks;
this trace review does not claim token savings.
