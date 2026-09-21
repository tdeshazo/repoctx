# M3-03 grounded relationship adapters

## Decision

Grounding is an explicit library stage over validated `repoctx.ir/v1alpha4` and
an optional artifact catalog. It does not alter either wire contract. Callers
choose adapters in `artifacts.GroundOptions`; repository content cannot activate
one. The stage performs no reads or process execution.

`GroundedRelationship.Resolution` separates the evidence classes:

| Resolution | Meaning |
| --- | --- |
| `declared` | A catalog relationship claim whose endpoints are unique locally |
| `declared_ownership` | An artifact's unverified owner label |
| `syntactic` | A repository-local Markdown destination observed in indexed syntax |
| `provider_resolved` | An exact same-module Go import resolved by the requested provider |

Existing context call edges retain their `name_heuristic` label and are not
copied into grounding output. No resolution class grants authority, establishes
transitivity, proves runtime dispatch, or verifies a declared owner.

## Providers and identities

The document-link adapter resolves exact file destinations already represented
by indexed Markdown reference edges. It retains fragments but does not validate
heading anchors. External URIs and email autolinks are outside coverage. A local
destination absent from the validated IR inventory is reported as unavailable;
the adapter does not claim that the file is absent from the repository.

The Go provider receives caller-supplied `go.mod` bytes and rejects them unless
their length and SHA-256 match the compilation manifest. It derives the module
path and repository package targets from those inputs, resolves exact
same-module import strings, and preserves explicit import aliases from the
indexed `ImportSpec`. External and standard-library imports are outside
coverage, and test files cannot become import targets. Build constraints, type
checking, and runtime dispatch remain outside coverage. Missing and ambiguous
local targets remain diagnostics.

Each provider reports a stable implementation version and an `InputID` over the
IR snapshot ID, provider version, and verified configuration. This identity makes
results reproducible; it is not source authentication. Indexed evidence sites
carry the verified file hash and line/UTF-8 byte-column span. Catalog sites also
retain their claimed byte ranges and remain `Verified: false`.

## Bounds and failure behavior

Relationship and diagnostic limits default to 4,096 and may be lowered through
`GroundOptions`; the production ceiling is 16,384 each. Omitted records are
counted separately and set `Incomplete`. Invalid repositories/catalogs, invalid
module paths, invalid limits, and parse-incomplete provider inputs fail without
partial results.

An empty request with a nil catalog succeeds with initialized empty slices. This
keeps compilation and context serving independent of catalogs, providers, the Go
tool, network access, or a build environment.
