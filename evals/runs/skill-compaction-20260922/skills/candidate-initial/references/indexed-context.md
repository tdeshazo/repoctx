# Indexed context and relationships

Read this when an exact source read cannot establish a relationship or when a
task needs semantic units or canonical manifest declarations. A manifest is
optional; validate an existing one only when its canonical metadata matters:

```sh
repoctx manifest -root /path/to/repo -file agent-context.yaml
```

Do not author a manifest for ordinary discovery. Apply `-allow`/`-deny` before
compilation when paths need scoping; `-deny` wins and does not scrub an existing
index. Put the index and temporary bundles in a caller-approved location
outside the repository unless a durable artifact was requested.

```sh
repoctx compile -root /path/to/repo -o /tmp/project.ir.json.gz
repoctx validate /tmp/project.ir.json.gz
repoctx context -root /path/to/repo -query 'exact API or component' \
  -depth 1 -max-bytes 12000 /tmp/project.ir.json.gz
```

An explicit context allow/deny policy must match compilation; omitted flags
inherit the authenticated index policy. Default verification checks declared
compilation inputs for drift, including optional `go.mod`; excluded files and
unsupported build configuration remain outside that contract. Recompile on a
stale-input or incompatible-contract error. Choose immutable consistency only
when the caller has established an isolated immutable tree and a snapshot pin.

Start with an exact API, failing-test name, path, error, or component. Every
flag precedes the final index argument. Inspect `warnings`, `omissions`,
`capabilities`, and relationship resolution labels. `name_heuristic` and
`unresolved` edges are leads, not proof of a call. Missing edges do not prove
absence. A clean index does not prove the code builds or tests pass.

For a caller/callee trace, start with an exact function or API name from source
and use only semantic IDs returned by context. `-direction in` selects callers;
`-direction out` selects callees. Verify each returned edge against its cited
callsite and report that site. Multiple calls from one caller are separate
edges, not an execution order. Read intermediate wrappers before following
them, and describe only the bounded chain the evidence establishes.

Context v1alpha3 uses IR v1alpha5 and returns source/profile/task identities.
Document and source `units` have `u:` IDs separate from symbol definitions;
reuse a returned unit ID with `-unit` and the previous `-expect-snapshot` to
expand it. Its parent ID may select a containing section or document. Check
`full_unit` versus `unit_excerpt`; a symbol `query_excerpt` is likewise only
selected body evidence. Query scores are inspection signals, not confidence.

For a targeted follow-up, use the returned semantic ID and snapshot:

```sh
repoctx context -root /path/to/repo -symbol 'RETURNED_SEMANTIC_ID' \
  -expect-snapshot 'sha256:ID_FROM_PREVIOUS_BUNDLE' \
  -direction out -depth 2 -max-bytes 12000 /tmp/project.ir.json.gz
```

If the snapshot check fails, recompile or investigate changed source before
using old evidence as current. Semantic IDs can change after renames or
collisions; dense numeric graph IDs are local to a snapshot. The byte cap is
for the entire uncompressed payload, not exact model tokens.
