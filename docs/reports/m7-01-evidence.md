# M7-01 canonical manifest evidence

M7-01 introduces `repoctx.manifest/v1alpha1`, a closed
`agent-context.yaml` contract for an authored namespace, source roots, artifact
sources, components, provider inputs, derived views, and supported capabilities.
The checked-in manifest is both the usage example and a Go dogfood fixture; the
published Draft 2020-12 schema receives independent shape tests.

`pkg/manifest` checks the 256 KiB input bound before YAML decoding, then rejects
multiple documents, aliases, anchors, explicit tags, implicit non-string
coercion, duplicate or unknown fields, missing fields, excess depth or entries,
unsafe paths, duplicate identities/references, unresolved references, and
unsupported contracts or capabilities. Validation returns no partial model and
does not quote repository-controlled values in diagnostics. The final load stage
uses the confined source-root reader to reject missing, symlinked, or wrong-type
declared directories and files without invoking a provider.

Authored types and the schema deliberately have no hash, coordinate, snapshot,
observed-state, authority, credential, or execution fields. Generated artifact
catalogs remain outputs; the manifest points to their semantic authoring input.
The `repoctx manifest` command emits the validated semantic model as JSON, and
`repoctx version` exposes the manifest contract alongside other current
contracts.

Dogfooding found that an initial CLI draft read `-file` relative to the process
working directory, which could place the manifest outside `-root`. The final
command instead uses `manifest.LoadFile`, requiring a safe repository-relative
path and the same symlink-resistant confinement as every declared input. The
finding was addressed within M7-01, so no duplicate follow-up item is needed.

Final verification passed all Go tests, the race detector, vet, gopls checks,
97 Python tests, the 39-check clean-distribution baseline, deterministic catalog
regeneration, schema validation, dependency checksum verification, and the
official Go vulnerability scan. The release check also validates the dogfood
manifest with the built executable and matches it to the advertised contract.
