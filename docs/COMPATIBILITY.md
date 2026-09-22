# Active-development contract policy

repoctx is in active, pre-1.0 development. Only the contracts emitted by the
current source tree are supported. Releases may make breaking changes to the Go
API, JSON shapes, identifiers, provider behavior, or stored indexes without a
compatibility period, migration tool, or deprecation cycle.

Contract identifiers remain explicit so a consumer can detect a mismatch and
rebuild instead of silently interpreting changed semantics. They do not promise
that repoctx will read an older version. Inspect the running executable with
`repoctx version -format json`; that output is descriptive, not proof that a
binary came from a trusted source.

## Current contracts

| Surface | Current identifier | Update action |
| --- | --- | --- |
| Public Go packages | `repoctx.go-api/v1alpha1` | Update callers and rebuild against the current module. |
| Repository IR | `repoctx.ir/v1alpha4` | Recompile the repository. |
| Repository entities | `repoctx.entities/v1alpha1` | Recompile with the current canonical manifest. |
| Agent context | `repoctx.context/v1alpha3` | Regenerate from a current IR index. |
| Live discovery | `repoctx.discovery/v1alpha1` | Rerun discovery. |
| Artifact catalog | `repoctx.artifacts/v1alpha1` | Regenerate from the maintained source declaration. |
| Artifact authoring | `repoctx.artifact-authoring/v1alpha1` | Update the declaration and regenerate the catalog. |
| Repository manifest | `repoctx.manifest/v1alpha2` | Update the manifest and rebuild all derived views. |
| Document frontmatter | `repoctx.frontmatter/v1alpha1` | Update declarations and recompile the repository. |
| Obligation handoff | `repoctx.obligations/v1alpha1` | Rebuild from current inputs and caller policy. |
| Document-link provider | `repoctx.document-links/v1` | Re-ground with the current provider. |
| Go-import provider | `repoctx.go-imports/v1` | Re-ground with the current provider. |
| Build information | `repoctx.build/v1alpha4` | Rerun `repoctx version`. |

Historical schemas and incidental legacy readers are development artifacts, not
supported compatibility surfaces. They may be changed or removed without
notice. The internal `repoctx.parse-fragment/v1alpha1` cache is also not a
public contract; a mismatch is a cache miss and should be recomputed.

## Change rules

Durable JSON schemas are closed with `additionalProperties: false`. Repository
decoders that accept serialized input reject unknown fields, unsupported
versions, extra JSON documents, and documented resource-limit violations.
Generic `json.Unmarshal` into an exported Go struct is not contract validation.

When a change alters a wire shape or the meaning of an ID, span, completeness
label, omission, capability, provider result, or trust field, change the
relevant identifier so stale consumers fail visibly. No translation from the
old version is required. Do not relabel an old payload, copy its manifest, or
recompute only its outer digest; rebuild derived data from current source and
caller-controlled inputs.

Application-owned roots, scope, authority, tokenizers, payload caps, execution,
and verification remain external. A new repoctx version cannot broaden those
permissions or turn repository evidence into trusted instructions.
