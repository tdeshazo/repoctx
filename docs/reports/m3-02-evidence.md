# M3-02 implementation evidence

Date: 2026-09-21. Scope: declaration authority analysis only.

M3-02 is implemented by `pkg/artifacts.Resolve`. Trusted caller configuration
explicitly supplies accepted IDs and allowed scopes. The resolver intersects
scope, emits deterministic diagnostics, and withholds ambiguity and conflicts.
It does not discover metadata, read sources, resolve providers, execute checks,
or alter the artifact wire contract, IR, context schema, CLI, or source-only
workflow.

The contract and decision rules are recorded in
[`docs/design/m3-02-authority-resolution.md`](../design/m3-02-authority-resolution.md).
Focused regression evidence is in `pkg/artifacts/resolve_test.go`, including:

- no effective artifact without explicit caller ID and scope;
- lifecycle claims do not add or remove authority;
- duplicate IDs, broken/ambiguous endpoints, invalid effective scopes, and
  missing accepted IDs remain visible;
- supersession cycles never pick a winner, and accepting both overlapping sides
  withholds both;
- conflicting accepted requirement inputs with overlapping scope withhold all
  affected requirements; and
- effective ordering and scope are independent of declaration array order.

Dogfooding used the checkout's discovery commands to locate the M3 contract and
artifact package. Three workflow findings were observed: an older `repoctx` on
`PATH` did not advertise the checkout's `discover` command; one broad lexical
query spent its bounded results on repeated high-overlap windows from the same
documents while marking the result incomplete; and an indexed Go query excerpt
began in the middle of a token before the selected function. These are recorded
as future roadmap work rather than silently folded into M3-02.

## Verification

- `go test ./pkg/artifacts ./pkg/agentctx`: passed.
- `go test -race ./pkg/artifacts ./pkg/agentctx`: passed.
- `gopls check` on all changed Go files: no diagnostics.
- `gopls vulncheck ./...`: no known reachable vulnerabilities reported on
  2026-09-21; this required the current external Go vulnerability database.
- `python3 scripts/m0_baseline.py --output
  /tmp/repoctx-m3-02-baseline-final.json`: 17/17 checks passed. The retained
  [baseline report](m3-02-baseline.json) has SHA-256
  `4b0ef01f1966ac8013cc8fdaa6eb08e9358d06e26e9bb4cdf5065900e4bf0835`.
  It covers the full Go tests, race tests, vet, both builds, deterministic
  compile/context/schema fixtures, Python tests, package builds, installation,
  and launcher behavior. The runner records a dirty checkout and source-tree
  digest rather than claiming a clean revision.
- Dogfood compile and validation over the scoped implementation and documents:
  `repoctx.ir/v1alpha4` valid, snapshot
  `sha256:d4f97a0dab9357ea31cd828ad7729c034e817e410e0c1abeeec950ea0ce6938e`.
  The focused query returned three symbols, three units, six evidence blocks,
  visible capability limitations, and explicit budget omissions.
- `git diff --check`: passed.

No completed M3 acceptance gate is claimed: integration into retrieval,
grounded providers, obligations, and model dogfooding remain M3-03 through M3-05.
