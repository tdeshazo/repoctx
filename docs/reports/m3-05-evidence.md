# M3-05 repository catalog dogfood

Date: 2026-09-21.

[`docs/repoctx-artifacts.json`](../repoctx-artifacts.json) describes repoctx's
compiler, discovery, context, artifact, and obligation packages; their canonical
wire contracts; the canonical development-verification requirement; two opaque
check IDs; and the active M4 evaluation milestone with M4-01 as its next gate.
It copies no instructions or command strings and is caller supplied, never
automatically discovered.

The root conformance test strictly decodes the catalog, verifies every source
hash and byte/line coordinate, resolves all endpoints under explicit whole-repo
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
Exact catalog hashes make roadmap edits fail conformance until provenance is
refreshed, which is the intended stale-evidence signal and needs no parallel
update workflow.

Verification:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `gopls check` on changed Go files
- `PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'`
- `git diff --check`
