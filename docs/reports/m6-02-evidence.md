# M6-02 conformance-fixture evidence

M6-02 is complete at the source state containing this report.

Dogfooding `repoctx discover` located substantial package-level coverage but no
single public-boundary fixture tying the current contracts together. The new
`conformance/testdata/repository` fixture is intentionally small and its derived
IR and context are built during tests rather than checked in as agent-authored
deterministic output.

`conformance/conformance_test.go` verifies:

- current IR write/read plus explicit stale-version and unknown-field rejection;
- equivalent JSON and Markdown evidence and omissions under final-payload caps;
- successful and failing caller tokenizer callbacks against serialized bytes;
- snapshot-pinned unit expansion, denied-source exclusion, and scope-drift
  rejection; and
- disabled versus selected relationship providers, exact provider versions,
  advertised coverage, content-derived input identities, and mismatched-input
  rejection.

`tests/test_m6_conformance.py` verifies that the model-neutral adapter preserves
the caller-owned binary, root, index, deny policy, payload cap, graph depth,
query, semantic IDs, unit IDs, and snapshot pin. It also rejects stale context
versions and output beyond the caller's cap. Existing package tests retain the
deeper negative and resource-bound cases; the conformance fixtures compose
those public guarantees without duplicating their full matrices.

Reproduce with:

- `go test ./...`
- `PYTHONPATH=src python3 -m unittest discover -s tests -v`
- `go run . artifacts -root . -source docs/repoctx-artifacts.source.json -o docs/repoctx-artifacts.json -check`
