# M3-05 repository catalog dogfood

Date: 2026-09-21.

[`docs/repoctx-artifacts.source.json`](../repoctx-artifacts.source.json) describes repoctx's
compiler, discovery, context, artifact, and obligation packages; their canonical
wire contracts; the canonical development-verification requirement; two opaque
check IDs; and the active M4 evaluation milestone with M4-01 as its next gate.
It copies no commands and is caller supplied, never automatically discovered.
Named source anchors are authored once; `repoctx artifacts` deterministically
produces the [strict wire catalog](../repoctx-artifacts.json), including hashes,
byte offsets, and physical coordinates.

The root conformance test first regenerates and byte-compares the catalog, then
strictly decodes it and resolves all endpoints under explicit whole-repo
authority, and rejects grounding diagnostics. For roadmap retrieval it excludes
retained reports and the test containing the prompt, then uses only the generic
prompt `next outstanding roadmap item`. It chooses the smallest returned unit
covering both catalog source spans, expands that unit under the same snapshot,
and builds the obligation sidecar. Both M4 and M4-01 retain `context_exact`
provenance. The prompt contains no expected roadmap prose, artifact ID, or
file/line answer.

Dogfooding reconfirmed the tracked M6-05 stale-PATH condition and found that the
root version test did not assert the newly advertised obligation contract; the
latter is corrected with this item rather than added as duplicate roadmap work.
Exact catalog hashes make roadmap edits fail conformance until the deterministic
generator refreshes provenance; agents do not calculate or author those fields.

Verification:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `gopls check` on changed Go files
- `PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'`
- `git diff --check`
