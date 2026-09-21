# M3-04 obligation handoff

## Decision

Keep obligation selection separate from `repoctx.context/v1alpha3`. The opt-in
`pkg/obligation.Build` API accepts an already built source-context result, a
caller-supplied catalog, and trusted `artifacts.Authority`. It emits
`repoctx.obligations/v1alpha1`, bound to the source task and snapshot. Existing
source-only compilation and context bytes do not change.

Only caller-accepted declarations whose effective file/subtree scopes contain a
selected context evidence path are emitted. Ambiguous, conflicting, or
out-of-scope declarations remain withheld by `artifacts.Resolve`; its bounded
diagnostics accompany the handoff. Declared relationships are emitted only when
both endpoints were selected. They retain `resolution: declared`, including
`verifies` links from verification obligations to requirements or contracts.

Artifact and relationship source spans retain their original whole-file hash and
coordinates. A source is `context_exact` only when one exact context evidence
range covers it and the file hash and physical coordinates agree. Otherwise it
remains `declared_unverified`; the handoff never reads an undeclared source to
upgrade that label.

`runner_check_id` is copied only from an applicable, accepted verification
obligation. It is an opaque lookup key. The wire contract deliberately has no
runner registration, command, environment, exit status, artifact-result, or
passing-result fields. A separate trusted runner may define and authenticate a
future result contract, but this stage does not execute it or infer an outcome.

The payload has independent artifact, relationship, diagnostic, and final-byte
limits with exact omission counts. Its content identity binds the source task,
snapshot, selected declarations, relationships, diagnostics, and omissions.
Pass it alongside—not re-encoded inside—the context payload so each byte budget
remains visible.

## Compatibility

This adds a standalone contract and leaves IR v1alpha4, context v1alpha3, the
CLI context command, and Python adapter unchanged. Catalog discovery remains
application-owned and opt-in. The schema is
[`docs/obligations.schema.json`](../obligations.schema.json).
