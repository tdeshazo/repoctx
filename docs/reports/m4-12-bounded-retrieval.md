# M4-12 bounded retrieval replay

Date: 2026-09-22

## Reproduced failures and decision

All three M4-09 incidents remained reproducible in the current discovery path.
Punctuated identifiers such as `M4-07` were split into ordinary terms, allowing
unrelated `m4` and `07` text to compete. Separately, matches on nearby generic
terms could merge context windows repeatedly until a result covered most of a
file. The serialized response then dropped that result whole when it did not fit.

Discovery retains identifier-like query fields intact. Review follow-up preserves
ordinary filename and prose candidates alongside identifier-centered windows;
identifier preference must not become a hard filter. Exact identifier matches
respect surrounding letter/digit boundaries, so `M4-07` does not receive an
exact-match preference for `M4-070`. Exact-match ties prefer documentation and
source over configuration inventory. Discovery
also bounds merged excerpts to one requested context window, preventing nearby
generic matches from turning a focused excerpt into a whole-file result. Search
and explicit reads retain their existing window behavior.

## Historical replay

The current executable was run against archives of each recorded source revision
with the original query and byte limit. Each response retained the required
`ROADMAP.md` evidence as an exact source slice and reported `output_limit` for
the remaining candidates.

| Incident | Revision | Limit | Retained roadmap span | Payload |
| --- | --- | ---: | --- | ---: |
| split `M4-07` identifier | `3a30dc0f` | 5,000 bytes | lines 439–445 | 4,643 bytes |
| `M4-08` window did not fit | `cfda41b` | 8,000 bytes | lines 449–455 | 7,833 bytes |
| `M4-09` action clipped | `bdd212b` | 12,000 bytes | lines 460–466 | 11,863 bytes |

Focused fixtures reproduce the generated-input noise, a densely matching file,
and an earlier planning mention followed by the actionable requirement at the
same 5,000-, 8,000-, and 12,000-byte limits. They verify the decisive text and
its half-open byte span against the source. A nearby identifier query and an
absent-identifier query cover local relevance and no-answer behavior.

This resolves the recorded bounded-retrieval failures. It does not establish an
improvement in downstream agent outcomes; that remains the purpose of M4-13.
