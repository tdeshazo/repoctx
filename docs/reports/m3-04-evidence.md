# M3-04 obligation handoff evidence

Date: 2026-09-21.

`pkg/obligation.Build` now emits a bounded `repoctx.obligations/v1alpha1`
sidecar bound to an existing context task and snapshot. Trusted caller authority
selects declarations by effective scope; authority conflicts remain diagnostics.
Exact context-covered declaration spans are distinguished from unverified source
claims, and declared `verifies` links retain their resolution label. Applicable
verification obligations expose only opaque runner check IDs. The schema has no
command, execution status, check result, or passing-result field.

Compatibility: source-only IR/context output, automatic catalog discovery, CLI
context serving, and the Python adapter are unchanged. The installed `repoctx`
preflight again found the already-recorded M6-05 stale-PATH condition, so no
duplicate roadmap finding was added; dogfooding used the checkout entry point.

Verification:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `gopls check` on changed Go files
- Draft 2020-12 schema check and representative-payload validation
- `git diff --check`
