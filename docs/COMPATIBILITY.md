# Compatibility policy

This document defines the supported compatibility surfaces for repoctx. Contract
identifiers name exact semantics, not a marketing release: changing the meaning
of an existing identifier without changing its version is prohibited.

The running executable reports its current contract set with
`repoctx version -format json`. That output is descriptive, not an authenticity
claim; callers must obtain the executable and source revision through a trusted
channel when provenance matters.

## Supported contracts and readers

| Surface | Current writer | Supported readers | Migration or rebuild |
| --- | --- | --- | --- |
| Public Go packages | `repoctx.go-api/v1alpha1` | Source consumers built against the current module | Follow deprecations and rebuild. An incompatible public API change requires a new Go API identifier. |
| Repository IR | `repoctx.ir/v1alpha4` | `compiler.Read` and `ir.Repository.Validate` accept `v1alpha2`, `v1alpha3`, and `v1alpha4`; context serving accepts only `v1alpha4` | Recompile `v1alpha2` and `v1alpha3` indexes before serving context. Never migrate by replacing the version string. |
| Agent context | `repoctx.context/v1alpha3` | Go validation accepts `v1alpha3`; the model-neutral Python adapter accepts the known `v1alpha1`, `v1alpha2`, and `v1alpha3` JSON shapes | Rebuild from a `v1alpha4` index. Historical schemas are retained for adapter conformance, not as writer targets. |
| Live discovery | `repoctx.discovery/v1alpha1` | Current schema only | Rerun discovery. Results describe a live bounded read and are not migrated. |
| Artifact catalog | `repoctx.artifacts/v1alpha1` | Current strict Go decoder and schema only | Regenerate from its maintained source declaration or update the declaration explicitly. |
| Artifact authoring | `repoctx.artifact-authoring/v1alpha1` | Current strict generator only | Update the source declaration, then regenerate the deterministic catalog with `repoctx artifacts`. |
| Obligation handoff | `repoctx.obligations/v1alpha1` | Current schema and writer only | Rebuild the handoff from current inputs and caller policy. |
| Document-link provider | `repoctx.document-links/v1` | Built-in exact name/version pair | Re-ground with a binary that advertises the required version. |
| Go-import provider | `repoctx.go-imports/v1` | Built-in exact name/version pair | Re-ground with a binary that advertises the required version. |
| Build information | `repoctx.build/v1alpha2` | Current diagnostic reader; `v1alpha1` is historical and lacks `contracts.go_api` and provider contract fields | Rerun `repoctx version`; build information is not a durable repository artifact. |

The internal `repoctx.parse-fragment/v1alpha1` cache is not a public contract.
An absent, invalid, or differently versioned cache entry is a cache miss and is
recomputed; users should not migrate cache files.

“Accepted” above means structurally readable for the named operation. It does
not promise that an older payload supports newer behavior. In particular,
legacy IR can be inspected by commands such as `stats` and `graph`, but it lacks
the compilation-input manifest required for source-verified context serving.

## Version and field rules

Durable JSON contracts are closed. Their schemas use
`additionalProperties: false`, and repository decoders that accept serialized
input reject unknown fields, unsupported versions, extra JSON documents, and
their documented resource-limit violations. Calling Go's generic
`json.Unmarshal` on an exported struct is not a conformance check; use the
repoctx reader or the matching schema and dispatch on `version` first.

Additive fields therefore require an explicit reader and schema decision; a
producer must not add a field and assume old readers ignore it. Removing or
renaming a field, changing its type, or changing its interpretation requires a
new wire version. Provider consumers must match both provider name and version
and respect the advertised coverage and limits. repoctx does not load unknown
providers or infer their capabilities.

The following meanings are immutable within a version:

- semantic and retrieval-unit IDs, snapshot IDs, and the snapshot-local nature
  of dense numeric references;
- byte ranges, line and UTF-8 byte-column coordinates, and half-open span rules;
- completeness, omission, warning, verification, and capability labels;
- trust boundaries, especially that repository declarations and evidence do
  not grant authority, execution permission, or authenticity.

A correction that changes one of those meanings requires a new wire or provider
version, migration notes, and conformance coverage. Historical payloads remain
interpreted according to their original version.

## Recompilation and migration

Recompile an index when its IR version is not the current writer version, when
the source or any recorded compilation input changes, or when the selected
compiler profile, grammar, scope, or input-accounting semantics change. A
version edit, copied manifest, or recomputed outer digest is not a migration and
does not establish current source semantics.

Context, discovery, grounding, and obligation outputs are derived evidence.
Regenerate them from current source/index inputs and the caller's current scope,
budget, and authority rather than translating IDs or trust labels in place.
When a supported historical reader exists, it is for inspection or an explicit
adapter path only; it does not upgrade the payload.

## Go API deprecation

The module remains pre-1.0. Additive exported APIs may retain the current Go API
identifier. A planned removal or incompatible signature or behavior change is
announced with a Go `Deprecated:` doc comment, a replacement when one exists,
and release notes for at least one tagged release before removal. Urgent
security or data-integrity fixes may shorten that interval and must be called
out in the release notes. Removal or any other incompatible public change
requires a new Go API identifier; normal Go module versioning rules still apply.

Experimental or internal surfaces are not covered unless a document explicitly
assigns them a contract identifier. Application-owned roots, policy, authority,
tokenizers, payload caps, execution, and verification remain external behavior
and cannot be made compatible by a repoctx payload.
