# M4-10 bounded discovery diversity

Date: 2026-09-21

## Failure reproduced

The packaged `repoctx 0.1.0` executable was run against this checkout with a
24,000-byte bound and the broad query:

```text
M3-02 artifact declarations authority duplicate IDs broken references invalid scopes supersession cycles conflicting accepted requirements
```

Before the change, the response retained six windows from five documentation
files. Two windows came from `docs/design/m3-02-authority-resolution.md`; the
first implementation result, `pkg/artifacts/resolve_test.go`, ranked seventh in
the unbounded result list and was removed by the output bound. The response
correctly reported `output_limit` and remained incomplete.

## Decision

Discovery still computes the same deterministic lexical rank. When the best
window matches at least four distinct terms, an at-most-eight-result coverage
prefix admits only candidates matching at least half as many distinct terms as
the best window. It selects the best result from each available evidence class
(documentation, configuration, or source), then the best results from distinct
files. All unselected results retain their prior relative order.

The relevance floor and short-query guard are deliberate: diversity does not
promote a weak source match merely because highly relevant documentation has
multiple answer-bearing windows. Search and explicit reads are unchanged.

## Evidence

- A focused unit fixture covers repeated documentation windows, source and
  configuration promotion, preservation of weak-result order, and the
  short-query guard.
- Full package tests, race tests, vet, and language-server diagnostics pass.
- Repeating the dogfood query after the change places implementation evidence at
  bounded rank two while retaining deterministic output and the same exact limit
  signals. The new report was denied in the after run so its query terms could
  not make the comparison self-referential.

| 24,000-byte dogfood result | Before | After |
| --- | ---: | ---: |
| Retained windows | 6 | 5 |
| Distinct files | 5 | 5 |
| First implementation rank | omitted (unbounded rank 7) | 2 |
| Duplicate retained file windows | 1 | 0 |
| Explicit `result_limit` / `output_limit` | yes / yes | yes / yes |

This is targeted regression evidence, not the held-out agent-quality evaluation
defined by M4-01 through M4-05. No general task-success improvement is claimed.
