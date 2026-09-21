# M3-02 authority resolution

Date: 2026-09-21. Status: implemented.

## Boundary

M3-02 adds a pure Go analysis stage after strict artifact decoding. The trusted
caller supplies exact accepted artifact IDs and allowed file/subtree scopes.
Repository declarations supply claims and provenance only. Empty caller lists
grant nothing, claimed lifecycle does not change acceptance, and a supersession
edge does not choose a winner. The resolver performs no filesystem, source,
provider, runner, or process access.

The output contains only unique declarations explicitly named by the caller and
only the representable intersections of declared and caller scopes. It also
contains deterministic, source-text-free diagnostics. Repository array order is
retained through declaration indexes for inspection but cannot decide authority.

## Diagnostic and withholding rules

| Condition | Diagnostic | Effective result |
| --- | --- | --- |
| Repeated artifact ID | `duplicate_id`; relationship endpoints also become `ambiguous_reference` | Ambiguous accepted ID is withheld |
| Absent relationship endpoint, including an unresolved external namespace | `broken_reference` | Relationship remains a non-effective declaration |
| Accepted ID absent from the catalog | `accepted_id_missing` | No artifact |
| Accepted declaration has no overlap with caller scope | `invalid_effective_scope` | Artifact is withheld |
| Unique supersession graph has a strongly connected component or self-loop | `supersession_cycle` | No winner is inferred |
| Caller accepts both overlapping sides of a supersession claim | `accepted_supersession_conflict` | Both are withheld |
| Accepted, overlapping requirements disagree on an input's role, requiredness, or two explicit pins | `conflicting_accepted_requirements` | Both are withheld |
| One accepted requirement contradicts itself on an input | `conflicting_accepted_requirements` | It is withheld |

An absent pin and an explicit pin are not treated as contradictory: absence is
unspecified rather than a competing digest. Duplicate identical input entries
remain non-conflicting declarations. Different requirements may apply to the
same scope when their declared inputs do not conflict; co-applicability alone is
not evidence of contradiction.

The current relationship model has no caller-authenticated external catalog, so
cross-namespace endpoints are explicitly unresolved. M3-03 may add a grounded
provider without changing this stage into an authority source.

## Trusted configuration

`artifacts.Authority` is outside the artifact wire format. Accepted IDs must be
unique, structurally valid, and local to the decoded catalog namespace. Caller
scopes use the existing exact `file` and `subtree` grammar and are bounded by
the existing applicability ceiling. Invalid trusted configuration returns an
error and no partial resolution. Artifact problems produce diagnostics so a
caller can inspect all independent declaration issues in one result.

The resolver defensively revalidates an exported, possibly mutated `Catalog`
before analysis. It neither reads a catalog filename nor discovers one by
convention. Existing source-only compilation and serving remain unchanged.

## Reproducible checks

Focused tests cover zero-authority defaults, lifecycle non-authority, exact
scope intersection, invalid effective scope, duplicate and broken references,
cycles, accepted supersession conflicts, cross/self requirement conflicts,
invalid trusted configuration, bounded diagnostics, and declaration-order
independence:

```sh
go test ./pkg/artifacts
go test -race ./pkg/artifacts
go test ./pkg/agentctx
```
