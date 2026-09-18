# M3-01: minimal optional artifact contract

Status: revised after DESIGN_REWORK; awaiting independent design review.
No supported schema or API yet.
Date: 2026-09-18. Owner: this workflow's implementation role.
Baseline: `69df7f840fa1c74212166bc64302c1b8c5e55333`.
Prerequisites: recorded M0, M1 and M2 completion, inspected as described in the
[evidence handoff](../reports/m3-01-evidence.md). Independent design review and
implementation evidence remain outstanding. This document does not complete M3.

## Decision and scope

Define a standalone, opt-in JSON declaration document with proposed version
`repoctx.artifacts/v1alpha1`. Keep it separate from the compact source index and
agent context. The public contract consists of the wire vocabulary, structural
validation rules and exact provenance coordinates below. Choose a Go package,
exported API names and an optional schema file only after design review; do not
reserve a repository manifest filename or add automatic discovery.

The first implementation should accept caller-supplied bytes and validate their
structure without filesystem access. It must not automatically read inputs,
resolve references, determine effective applicability, or invoke commands.
Repository metadata is untrusted data even when structurally valid. This is
M3-01 only: authority/conflict resolution belongs to M3-02, grounded adapters to
M3-03, obligation selection/results integration to M3-04, and dogfooding to M3-05.

JSON avoids requiring a YAML parser, aliases, tags or interpolation. The YAML in
the roadmap remains illustrative and is not accepted configuration. Model a
verification obligation as an artifact kind, rather than a second identity table.

## Envelope and scalar rules

All objects are closed: reject unknown properties, duplicate JSON member names,
null values, trailing documents, invalid UTF-8, unpaired Unicode surrogate
escapes, and wrong types. Optional means absent, not null. Arrays are required
where listed and may be empty unless a minimum is specified. Do not silently
coerce strings, integers, paths or enums. No extensions bag in this version.

| Envelope field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `version` | string constant | yes | `repoctx.artifacts/v1alpha1` |
| `namespace` | namespace string | yes | Repository-declared identity namespace, not authenticated ownership |
| `artifacts` | array of artifact objects | yes | Declarations, including historical ones |
| `relationships` | array of relationship objects | yes | Directed, explicitly declared claims |

A namespace matches `[a-z][a-z0-9-]{0,62}`. An artifact ID is
`<namespace>:<local-id>`; local IDs match `[a-z][a-z0-9._-]{0,127}`.
For example, `repoctx:requirement.exact-evidence`. Each artifact's namespace must
equal the envelope namespace. Relationship endpoints use the same ID grammar;
other namespaces are allowed as unresolved declarations. IDs are case-sensitive
ASCII and never interpreted as URIs, paths, commands or executable names.

Authors assign IDs once and preserve them across moves, heading changes and
ordinary edits. Do not derive them from positions, hashes, dense graph IDs or
artifact kind. Never reuse a retired ID for a different concept; an incompatible
replacement gets a new ID and may declare `supersedes`. Namespace renames change
identity and need explicit migration by consumers. Cross-repository uniqueness
and historical non-reuse cannot be proved by schema validation. A consumer must
bind IDs to its caller-selected repository/generation, not trust a namespace as
global identity. Duplicate artifact IDs remain representable in the declaration
array; diagnosing ambiguous IDs and resolving references is M3-02. No reader may
silently overwrite duplicates in a map or use first/last entry as authority.

## Artifact fields

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | artifact ID | yes | Stable author-assigned identity |
| `kind` | enum string | yes | `component`, `decision`, `contract`, `requirement`, `verification_obligation` |
| `owner` | string, 1–256 UTF-8 bytes | no | Claimed person/team label; absence means unspecified; no account lookup |
| `applies_to` | array of applicability objects | yes | Zero or more claimed path extents; empty means unspecified, never whole repository |
| `lifecycle` | enum string | yes | `draft`, `active`, `deprecated`, `superseded`, `retired`; all are claims |
| `sources` | array of source-span objects, minimum 1 | yes | Exact locations supporting the declaration, not copied summaries |
| `declared_inputs` | array of input objects | yes | Claimed dependencies; empty makes no completeness claim |
| `runner_check_id` | string, 1–256 ASCII characters | no | Opaque check key, allowed only for `verification_obligation` |

`runner_check_id` matches `[A-Za-z0-9][A-Za-z0-9._:/-]*`. It is not a command,
provider activation request, executable path or proof of a registered runner.
Obligations can omit it when no runner mapping exists. A declared `verifies`
relationship connects an obligation to its requirement/contract. Endpoint kind
checking and resolving that relationship are deferred. There are no command,
shell, environment, credentials, permission, approval, execution-status or
passing-result fields. Lifecycle `active` does not mean accepted by a caller;
`superseded` does not establish an effective replacement or precedence.

An applicability object has exactly two required fields: `kind` is `file` or
`subtree`; `path` is a repository-relative path. A file denotes that exact path;
a subtree denotes descendants at a slash boundary. Only a subtree can use `.`
to claim the whole repository. No globs, negation, environment expansion or
implicit inherited scope. These extents describe claims; later applicability
resolution must intersect them with caller-controlled policy. Their presence
does not authorize enumeration or reading.

Paths are nonempty UTF-8 slash paths, at most 4096 bytes. Reject absolute paths,
empty segments, `.`/`..` segments (except the subtree root above), trailing slash,
backslash, colon, ASCII controls including NUL/DEL, and glob metacharacters
`*?[]{}!`. Preserve spelling, case and Unicode normalization; do not URL-decode,
case-fold or filesystem-normalize them. Filesystem confinement and existence are
separate checks, not consequences of this lexical grammar.

## Exact source provenance

Every source-span object has all of these fields and no others:

| Field | Type | Meaning |
| --- | --- | --- |
| `path` | file path | Repository-relative source file |
| `sha256` | string matching `[0-9a-f]{64}` | Digest of the entire original file |
| `start_byte`, `end_byte` | nonnegative integers | Half-open absolute byte offsets |
| `start_line`, `end_line` | positive integers | One-based physical source lines |
| `start_byte_column`, `end_byte_column` | nonnegative integers | Zero-based UTF-8 byte columns; exclusive end |

Integers must use JSON integer notation without fraction or exponent and fit
`0..9007199254740991`; line fields start at 1. Require `start_byte < end_byte`
and lexicographically increasing `(line, column)` positions. Empty/synthetic
spans are invalid. Source-byte verification later checks both representations
agree, ranges fit the file, boundaries align to UTF-8, and the full hash matches.
Lines split on LF; CRLF occupies two bytes and CR is not stripped. Ignore `//line`
remapping. EOF is valid as an exclusive endpoint, including line-after-final-LF
column zero. Noncontiguous evidence requires separate spans. No reconstruction,
newline normalization or synthesized prose may masquerade as exact evidence.

A supplied hash and span are assertions until checked against caller-permitted
bytes. Structural validation alone must never label them verified. Sources may
point to the catalog's own original bytes or another file; verification must use
those original bytes, not re-serialized JSON. A self-referential whole-file hash
cannot be manufactured: prefer external source documents or have callers retain
the declaration document's location/hash separately as transport provenance.
No catalog-level self-hash is required.

## Declared inputs and relationships

An input object requires `path` (file-path string), `role` (enum `source` or
`configuration`), and `required` (boolean). It optionally carries `sha256`
(the same 64-hex grammar), an author-declared content pin. It contains no observed
availability state. `required: true` claims that the artifact depends on that
input as a necessary prerequisite; `required: false` claims an optional dependency
whose absence does not, by itself, make the artifact's declared prerequisites
unsatisfied. Neither value specifies execution, asserts availability, establishes
an effective requirement, or grants permission to read. A declared pin does not
prove that the file was read. Inputs are descriptive dependencies, not
instructions to open files. Do not merge them into M2's observed `InputManifest`.

Preserve every input entry in array order, including repeated paths, identical
entries, and entries with conflicting pins, roles or requiredness. These are
structurally valid declarations; do not deduplicate, merge or select a winner.
Each occurrence counts toward input and aggregate limits. Effective dependency
and conflict resolution remain M3-02.

### Declared-input design fixtures

The following JSON is a fixture fragment for an otherwise valid artifact's
`declared_inputs`. It is expected to be structurally accepted with all five
entries preserved exactly in order; pins are syntactic assertions only.

```json
[
  {"path":"config.json","role":"source","required":true,"sha256":"0000000000000000000000000000000000000000000000000000000000000000"},
  {"path":"config.json","role":"source","required":true,"sha256":"0000000000000000000000000000000000000000000000000000000000000000"},
  {"path":"config.json","role":"source","required":true,"sha256":"1111111111111111111111111111111111111111111111111111111111111111"},
  {"path":"config.json","role":"configuration","required":true,"sha256":"0000000000000000000000000000000000000000000000000000000000000000"},
  {"path":"config.json","role":"source","required":false,"sha256":"0000000000000000000000000000000000000000000000000000000000000000"}
]
```

Required fixture variants for implementation:

| Variant | Structural expectation / declared meaning |
| --- | --- |
| First entry alone | Accepted; necessary prerequisite with an unverified pin |
| Last entry alone, remove `sha256` | Accepted; optional dependency, unspecified pin and availability |
| Empty array | Accepted; no completeness claim |
| All five entries above | Accepted; exact duplicate and independent pin/role/requiredness conflicts preserved |
| Remove `required` from first entry | Rejected; no default requiredness |
| Replace `required` with `null`, `"false"` or `0` (separate cases) | Rejected; boolean required |
| Replace `role` with `optional`, or shorten `sha256` (separate cases) | Rejected; requiredness is separate from role and pin grammar |

These are design fixtures, not executed decoder conformance results. Acceptance
must cause no filesystem reads or checks, including for unavailable inputs.

Each relationship requires `from` and `to` (artifact IDs), `kind` (enum below),
`resolution` (constant `declared`), and `sources` (1 or more source spans).

| Kind | Direction of claim |
| --- | --- |
| `contains` | Container to contained artifact |
| `references` | Referring artifact to referenced artifact |
| `governed_by` | Component/artifact to requirement or contract |
| `depends_on` | Dependent artifact to declared dependency |
| `supersedes` | Replacement to historical artifact |
| `verifies` | Verification obligation to requirement or contract |

No inference follows from an edge, including transitivity, ownership, authority
or successful verification. Preserve edge arrays without deduplication or
precedence rules. Missing/ambiguous endpoints, self-edges, cycles and contradictory
claims can be structurally valid; they need M3-02 diagnostics before effective use.
Other resolution values, provider identities, symbol endpoints and executable
adapters are not part of this minimal contract. Existing syntactic and heuristic
IR/context edges retain their current meanings.

## Bounded decoding and validation stages

The proposed reader accepts at most 1 MiB of uncompressed input, including
whitespace; no built-in archive, gzip or remote fetching. Enforce limits before
unbounded allocation: maximum JSON nesting depth 16, 1024 artifacts, 4096
relationships, 64 applicability entries and 64 inputs per artifact, 16 source
spans per artifact/relationship, and 16384 combined nested entries across all
arrays other than the two envelope arrays. Limits are inclusive, and exceeding
any limit fails the whole document without returning a usable partial catalog.
String limits count decoded UTF-8 bytes. Caller limits may be lower, never higher
than these ceilings. Bounded structural errors identify the field/index and
reason, without echoing source text or arbitrary owner/path values.

### Achievable resource evidence

Do not claim that every hard ceiling can be reached by an accepted wire document.
The byte cap precedes structural validation. For example, a compact catalog with
namespace `a`, no artifacts and 4096 minimal `contains` relationships from `a:a`
to `a:a`, each with a minimal source span at path `a`, is 1,093,721 bytes. It
exceeds 1,048,576 bytes before the inclusive relationship ceiling is reached.
Depth 16 likewise cannot be an accepted catalog: the closed wire shape has a
maximum container depth of 5 (root object is depth 1).

For every limit, record the tested layer, configured limit, observed size/count,
expected outcome and actual rejection reason. Use these complementary checks:

| Layer | Required boundary evidence |
| --- | --- |
| Reader, reachable hard boundaries | Otherwise valid documents at and one over byte cap (pad an empty envelope with whitespace), artifact count, per-artifact applicability/input counts, per-object source counts, and scalar length/range limits; keep all other limits below ceilings |
| Reader, lower caller limits | For relationship count use limit 2 with 2 accepted and 3 rejected relationships; for combined nested entries use limit 2 with one artifact containing one source plus 1 accepted / 2 rejected inputs. Both pairs fit the byte cap. Exercise other configurable counters similarly where a hard boundary is masked |
| Isolated structural validator | Construct in-memory declarations at 4096/4097 relationships and 16384/16385 combined nested entries, respecting per-object limits. Test the actual production counter logic without the byte reader; do not expose a public bypass or raise reader ceilings |
| Isolated bounded tokenizer | Nested arrays at depth 16/17 test the production depth guard independently of catalog shape; these are tokenizer cases, not accepted catalogs. Reader depth tests also accept a valid depth-5 catalog with caller limit 5 and reject the same catalog with limit 4 |
| Reader, masked boundaries | Oversized relationship documents and shape-invalid deeply nested JSON must be rejected without partial catalogs. Record byte/shape/depth rejection actually observed; an earlier rejection does not prove a later counter ran |

Count depth as simultaneously open object/array containers, including root.
The future evidence must identify which hard boundaries were accepted through
the reader and which were tested only with lower limits or isolated production
validation. If a generated case hits an earlier limit, reclassify it as masked
and add an isolated or lower-limit pair; never report it as accepted-boundary
coverage. This does not require M3-02–M3-05 implementation.

Keep validation stages explicit:

1. M3-01 structural decoding: version, fields, types, scalar grammar, local
   span ordering, enum constraints and resource limits. No external I/O.
2. Later source grounding: caller-authorized original bytes, hashes, coordinate
   agreement and input identity. No automatic reads during structural validation.
3. M3-02 authority/reference handling: duplicates, broken references, invalid
   effective scopes, supersession cycles and conflicting accepted requirements.
   No file-order conflict resolution and no promotion of claims to instructions.
4. M3-03/M3-04 integration: grounded providers and selected obligations under
   explicit contracts. A runner alone produces authenticated execution results.

A future JSON Schema should express the structural subset using Draft 2020-12,
with runtime checks for duplicate JSON keys, integer spelling, byte lengths,
aggregate limits and cross-field conditions the schema cannot express alone.
Schema conformance is neither source verification nor normative consistency.

## Compatibility and affected surfaces

There is no IR/context wire change: keep `repoctx.ir/v1alpha4` and
`repoctx.context/v1alpha3`, their validators and their cache identities unchanged.
No artifact fields are attached to existing closed-schema objects. No recompile
or migration is required for source-only users. With no explicit catalog bytes,
compile and serve behave exactly as before; no metadata file is required and
the presence of a similarly named file never activates a feature.

Unknown artifact versions, kinds and fields fail explicitly; future additions
require a new artifact version and migration notes. Do not rewrite version
strings on existing data. Catalog array order is preserved for inspection but
does not determine authority. This version defines no semantic catalog digest
or persistent cache: raw bytes can be hashed for provenance, not authentication.

Later integration must bind catalog bytes, grounded source/input states,
implementation identity and caller scope to a separately reviewed generation
identity. It must not serve artifact-enriched evidence under unchanged M2 task
keys. Persisting inputs in IR or adding artifacts to context requires explicit
version/migration decisions at that time. Denied source metadata and catalog
declarations remain sensitive; the caller must authorize the catalog itself.

The next M3-01 implementation would add a standalone schema, a bounded decoder
and structural validator, documentation and fixtures in locations selected after
review. Existing `pkg/ir.Repository`, `compiler.Compile`/`Read`/`LoadInputs`,
`agentctx.Build`, selection/rendering, CLI, Python launcher and source-root access
need no change. This iteration changes only this design and its evidence handoff.

## Acceptance matrix for the next implementation

These are planned checks, not test results or completed roadmap gates.

| Area | Required evidence |
| --- | --- |
| Minimal vocabulary | Accepted fixture for every artifact kind; all required/optional fields; empty envelope arrays; obligation with/without opaque check ID |
| Identity | Stable ID through file relocation; invalid namespace/ID rejection; duplicate artifact IDs retained as declarations, with no map overwrite |
| Parsing | Wrong/missing types, nulls, unknown fields/enums/versions, duplicate JSON keys, trailing JSON, invalid UTF-8/surrogates and non-integer coordinates rejected |
| Provenance | Structural span ordering failures; independently checked UTF-8, CRLF, EOF and separated-span fixtures; distinguish syntactic acceptance from verified bytes |
| Scope | Path grammar and explicit root subtree; no glob/command interpretation; validation with nonexistent or denied paths causes zero reads |
| Claims | All lifecycle/relationship kinds round-trip; dangling/cyclic/conflicting claims do not gain authority; result/command/provider fields rejected |
| Declared inputs | Execute the design fixture and variants above; preserve repeated paths and conflicts without resolution or reads |
| Resources | Follow the layered resource evidence plan above; distinguish reachable accepted boundaries from masked limits; at/over counter checks with lower caller limits or isolated production validation; no partial catalogs and bounded diagnostics |
| Compatibility | Existing source-only compile/build outputs unchanged; same-named metadata does not activate loading; current IR/context schemas unchanged |
| Reproducibility | Deterministic parsing and preservation of declarations independent of host path/time; no new cache or identity claims |

Before marking M3-01 complete, retain the reviewed design, implementing revision,
positive/negative fixture results and focused regressions. Full M3 retrieval,
authority, provider and outcome acceptance gates remain deferred.
