# M3-03 implementation evidence

Date: 2026-09-21. Scope: grounded relationship adapters only.

`pkg/artifacts.GroundRelationships` now preserves declared catalog edges and
ownership separately from indexed Markdown references and opt-in, provider-
resolved Go imports. Results include provider version, content-derived input
identity, explicit resolution method, coverage limits, and source sites.

Focused fixtures cover:

- deterministic declared, ownership, syntactic, and provider-resolved output;
- repository-local Markdown links with retained fragments, ignored external
  links, and explicit missing-target diagnostics;
- exact same-module Go imports, explicit import aliases, ignored external imports,
  and explicit missing-target diagnostics;
- source-only empty operation, invalid provider configuration, parse-incomplete
  inputs, lowered output limits, omission counts, and exact incompleteness; and
- absence of filesystem, network, subprocess, Go-tool, plugin, or check execution.

Dogfooding used `repoctx discover` to locate the next unchecked item and the
existing Markdown/Go compiler edges, then exercised the new API against this
checkout. The final run reported two providers, 128 relationships, 35 local link
destinations unavailable in the validated IR inventory, no Go-provider target
diagnostics, and no limit omissions. It also found that `next-roadmap-item.yaml`
and its workflow document remain pinned to completed M3-01; M6-06 records that
automation-maintenance gap.

## Verification

- `gopls check pkg/artifacts/ground.go pkg/artifacts/ground_test.go pkg/artifacts/types.go`
- `gopls vulncheck ./...` (no known reachable vulnerabilities)
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `python3 -m unittest discover -s tests -p test_artifacts_schema.py -v`
- `python3 scripts/m0_baseline.py --output /tmp/repoctx-m3-03-baseline.json`
  (`17/17` checks passed)
- `go run /tmp/m3_03_dogfood.go` (`2` providers, `128` relationships,
  `35` document-inventory diagnostics, `0` omissions)
- `git diff --check`

The adapters do not yet attach obligations to context bundles (M3-04), dogfood a
repository-authored catalog (M3-05), validate Markdown fragments, resolve external
packages, apply build tags, type-check calls, or prove runtime dispatch.
