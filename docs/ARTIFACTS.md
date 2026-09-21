# Optional artifact declarations

`pkg/artifacts.Decode(data, artifacts.Limits{})` accepts caller-supplied JSON
under `repoctx.artifacts/v1alpha1`. It returns a `*Catalog` or a nil catalog with
a bounded error. Decoding adds no reserved catalog filename, discovery, source
reading, source verification, command execution or context enrichment.
The [accepted design](design/m3-01-artifact-schema.md) defines the contract.

The model preserves declaration array order and duplicates, including conflicting
input claims and duplicate artifact IDs. `json.Marshal(catalog)` round-trips this
model; whitespace and JSON escape spelling are not preserved. Optional owner,
input hash and runner check ID use pointers to retain absence; required empty
arrays remain arrays. Newly constructed Go values must satisfy the wire contract
before a caller can decode their serialization; marshaling alone is not validation.

Every limit is inclusive. Zero selects its production ceiling, positive values
lower it, and negative or above-ceiling settings fail. Production ceilings are
1 MiB including whitespace, depth 16, 1024 artifacts, 4096 relationships, 64
applicability claims and inputs per artifact, 16 sources per object, and 16384
combined nested array entries. The byte guard runs before parsing or allocations
based on input content; syntax scanning checks depth and duplicate decoded keys
before object-shape decoding. Subsequent allocation and validation are bounded by
that byte ceiling. Errors identify trusted field/index paths, syntax container ordinals (`[index]` for arrays,
`{index}` for object members), or the JSON root and
never echo declaration values or unknown member names.

The [Draft 2020-12 schema](artifacts.schema.json) expresses the structural subset.
Use the decoder for strict JSON spelling, Unicode, duplicate keys, decoded UTF-8
byte lengths, namespace equality, span ordering, and aggregate/transport limits.
Schema validity and decoder acceptance both leave hashes, coordinates, ownership,
lifecycle, dependency claims and check IDs unverified. Source-byte verification
remains a separate, deferred stage.

## Resolve trusted authority

`pkg/artifacts.Resolve(catalog, artifacts.Authority{...})` performs the M3-02
authority stage without filesystem or process access. Both `AcceptedIDs` and
`Scopes` are trusted caller inputs. An empty list grants nothing. Effective
scope is the intersection of each accepted declaration's `applies_to` claims
and the caller scopes; repository content cannot add an ID or widen that
intersection. Lifecycle values remain labeled claims: `active` does not grant
authority, and `retired` does not override an explicit caller choice.

Resolution reports duplicate IDs, missing or ambiguous relationship endpoints,
invalid effective scopes, supersession cycles, simultaneously accepted sides of
a supersession claim, and accepted requirements with contradictory declarations
for the same input path. Ambiguous or conflicting declarations are withheld
from the effective result. Supersession never selects a winner, and declaration
array order does not determine authority. Relationships remain unresolved
repository claims for later grounded adapters; resolution does not turn a
`runner_check_id` into a command or registered check.

Trusted authority is validated independently and bounded by the artifact and
scope ceilings. IDs must be local to the decoded catalog namespace, scopes use
the same exact file/subtree grammar, and duplicate accepted IDs are caller
errors. Diagnostics contain stable codes and declaration indexes without
copying repository prose. Callers should treat `Resolution.Artifacts` as scoped
effective declarations and retain `Resolution.Diagnostics` with the original
catalog for inspection.

Run conformance and source-only compatibility checks with:

```sh
go test ./pkg/artifacts ./pkg/agentctx
python -m unittest discover -s tests -p test_artifacts_schema.py -v
```

The shared positive fixture and original source bytes are under
`pkg/artifacts/testdata/`. Go tests separately exercise strict decoder rules and
reachable/lower/isolated/masked resource boundaries. The Linux-specific inotify
test detects actual access to declared files and checks that an executable opaque
check ID is never run. Other platforms retain all portable structural tests.

## Ground repository relationships

`pkg/artifacts.GroundRelationships` is an optional, non-executing M3-03 stage
over a validated IR repository and an optional catalog. With no catalog or
adapters requested it returns an empty, complete result, preserving the normal
source-only workflow. Repository declarations cannot activate either adapter.

Catalog relationships with unique local endpoints remain `declared`; owner
labels become separate `declared_ownership` claims. Missing and duplicate
endpoints produce diagnostics instead of edges. Neither form inherits the
caller authority accepted by `Resolve`.

The caller may explicitly request two bounded adapters:

- document links resolve indexed Markdown destinations to exact repository files
  as `syntactic` references; external URIs remain outside coverage, local
  destinations absent from the IR are reported as unavailable rather than
  asserted absent, and fragments are retained without claiming the heading exists;
- the pilot Go-import provider accepts caller-supplied `go.mod` bytes, verifies
  them against the compilation manifest, and resolves exact same-module import
  declarations as `provider_resolved` dependencies. It preserves explicit
  aliases and diagnoses missing or ambiguous package targets. Test files are not
  import targets. Standard-library/external imports, build tags, types, and
  runtime dispatch are outside its advertised coverage.

Each requested provider reports its implementation version, a content-derived
input identity bound to the IR snapshot and verified configuration, resolution
kinds, and coverage limits. Relationship sites carry indexed file hashes and
source coordinates; declared sites remain visibly unverified claims. Parse-
incomplete inputs fail explicitly. Caller-selected relationship and diagnostic
limits produce exact omission counts and `Incomplete`, never silent truncation.

The stage performs no filesystem, network, Go-tool, plugin, or process access.
It does not turn heuristic call edges into provider-resolved dependencies and
does not prove that an import is used at runtime.

## Select verification obligations

`pkg/obligation.Build` creates an opt-in `repoctx.obligations/v1alpha1` handoff
from an existing `agentctx.Result`, a catalog, and trusted caller authority. It
selects only accepted declarations whose effective scope contains source
evidence in that context task, retains authority diagnostics, and binds the
result to the context task and snapshot. The standalone
[JSON Schema](obligations.schema.json) leaves context v1alpha3 unchanged.

Artifact and relationship spans are labeled `context_exact` only when the
existing context evidence covers the exact range with matching hash and physical
coordinates; all other provenance remains `declared_unverified`. Declared
`verifies` relationships stay declared claims. An applicable verification
obligation may expose its opaque `runner_check_id`, but the handoff cannot
register or execute it and contains no command, exit status, check result, or
passing status. See the [M3-04 decision](design/m3-04-obligation-handoff.md).

## Repository dogfood catalog

[`repoctx-artifacts.source.json`](repoctx-artifacts.source.json) is the maintained
dogfood declaration. It records semantic declarations and named, unique source
anchors once; it contains no hashes, byte offsets, line coordinates, or copied
commands. `repoctx artifacts` resolves those anchors beneath an explicit root
and deterministically emits the strict
[`repoctx-artifacts.json`](repoctx-artifacts.json) wire catalog:

```sh
repoctx artifacts -root . -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json
repoctx artifacts -root . -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json -check
```

The authoring input uses `repoctx.artifact-authoring/v1alpha1`. Start and end
anchor text must each occur exactly once in a UTF-8 regular file; equal values
select that single occurrence. Files are read through the same root-confined,
symlink-rejecting reader used for source evidence. Named anchors prevent repeated
provenance declarations. Generation preserves declaration order, computes the
whole-file SHA-256 and exclusive physical span, validates the emitted artifact
contract, and atomically writes only after success. Its stable line-oriented
JSON keeps each declaration independently retrievable. `-check` performs no
write and fails when the committed output differs.

Neither file is reserved, discovered, or loaded automatically. The root dogfood
test regenerates and byte-compares the catalog before decoding it, resolving all
endpoints under explicit caller authority, and exercising catalog-guided
progressive disclosure of the current incomplete milestone and its next unmet
gate. Editing a referenced file therefore makes the generated-output check fail
without requiring an agent to calculate provenance or maintain a second roadmap
or command registry.
