# M7-02 typed repository entities

Completed 2026-09-22.

`repoctx compile -manifest agent-context.yaml` now attaches a
`repoctx.entities/v1alpha1` model to the existing repository IR. The model
contains namespace-qualified, ID-sorted documents, components, decisions,
contracts, requirements, verification obligations, and owners. Every entity
retains owners, file/subtree scopes, lifecycle, supersession, sensitivity,
freshness inputs, status `declared`, and a full-file hash plus exact physical
byte/line range for its authored declaration.

The manifest contract is now `repoctx.manifest/v1alpha2`. It explicitly lists
Markdown inputs using `repoctx.frontmatter/v1alpha1`; no document is promoted
implicitly. The compiler resolves owner and same-kind supersession references,
rejects duplicate identities and malformed or excessive input, intersects all
declared file reads with caller scope, invokes no provider, and grants no repository claim
authority. Omitting `-manifest` retains the previous source-only syntax graph.

The checked-in manifest and `docs/MANIFEST.md` dogfood all seven entity kinds.
Generated artifact hashes and coordinates were refreshed with `repoctx
artifacts`, not authored manually.

Validation:

- `go test ./...`
- `go vet ./...`
- `uv run --extra test python -m unittest discover -s tests -v`
- `repoctx manifest -root . -file agent-context.yaml`
- `repoctx compile -root . -manifest agent-context.yaml -o /tmp/repoctx-m7-02.ir.json`
- `repoctx validate /tmp/repoctx-m7-02.ir.json`
- JSON Schema validation for manifest, frontmatter, and the optional IR entity layer
