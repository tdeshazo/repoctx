# M4-11 indexed excerpt boundaries

Date: 2026-09-21

## Failure and decision

M3-02 dogfooding produced a `query_excerpt` whose exact source span began in the
middle of a Go token. The file hash, half-open byte coordinates, completeness
label, and source bytes were correct, but the presentation made the evidence
harder to read.

Query-centered symbol excerpts now begin at the highest-coverage source line
when that line can retain the query within the current excerpt window. Trailing
context ends at the last complete line after the match when possible. This
changes selection boundaries only: evidence remains an unmodified source slice,
and the existing final serialized byte cap remains authoritative.

A pathological source line may itself exceed every attempted excerpt size. In
that case retrieval still retains the query and emits a warning that exact
evidence uses partial source-line boundaries; the non-zero UTF-8 byte columns
identify those boundaries. The warning is derived after evidence coalescing so
it cannot cite a stale evidence range.

## Dogfood replay

The current checkout was compiled and queried with an explicit `Resolve` seed,
the terms `accepted conflict scope supersession`, depth zero, and a 6,000-byte
cap. The selected `pkg/artifacts/resolve.go` query excerpt:

- begins at line 66, byte column 0;
- starts with the complete source line containing the selected call;
- retains `query_excerpt` completeness and exact source coordinates; and
- produces a 5,791-byte final JSON payload, below the requested cap.

Focused regressions cover a late decisive branch, exact evidence fidelity, a
line-aligned start, and the warning fallback for a single overlong source line.
This is presentation-level regression evidence, not a claim that agent task
quality improved.
