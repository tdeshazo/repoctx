# M3-01 accepted implementation evidence

Date: 2026-09-18. Owner: this workflow's implementation role.
Design verdict: **accepted for implementation**, from the supplied independent
review of `252686b`. Completion verdict: **ACCEPTED — the whole M3-01 item**, from the supplied
[final independent review](m3-01-final-review.md). No unresolved in-scope
findings or external blockers remain. M3-02–M3-05 remain deferred.
No material wire-contract change was made; any future material change must return
through design review before implementation.

## Scope and revisions

Entry HEAD: `252686b` (`docs(m3-01): clarify input claims and resource evidence`).
That revision contained only two Markdown documents. Its previous design-rework
status is superseded by the supplied review, not by a self-issued design approval.
Earlier prerequisite baseline: `69df7f840fa1c74212166bc64302c1b8c5e55333` (M2).

The implementing revision is `eedacb52f2d0faa60224e8aabc5cbc9f03cac816`
(`feat(artifacts): implement bounded declaration contract`). It contains the new
`pkg/artifacts` package, schema, tests, API documentation and pre-run handoff. This report
was persisted before the subsequent baseline-runner tool step. No implementation
was present at entry; all implementation evidence below is from this iteration.

Reviewed scope: standalone `repoctx.artifacts/v1alpha1` model, strict bounded
`Decode([]byte, Limits)`, Draft 2020-12 schema subset, API documentation,
independent fixtures, decoder/schema tests, and source-only regression tests.
The Go API and file locations were implementation choices explicitly left open
by the accepted design. No new module dependency was added.

No repository or ancestor AGENTS.md applies. Go routing, style, error handling,
documentation and conventional-commit skills were used. The referenced optional
Go testing skill was not installed; no skill-specific test result is claimed.
The final independent review accepted the implementation and evidence against
the accepted contract; see [final review](m3-01-final-review.md).

Preserved pre-existing untracked files: `docs/workflows/next-roadmap-item.md` and
`next-roadmap-item.yaml`. They are excluded from commits. Only M3-01 is now checked in ROADMAP.md. Existing
IR/context schemas, source compilation, context production, CLI, Python launcher,
and the baseline runner are unchanged.

## Finding dispositions

| Supplied finding | Implementation and evidence | Disposition |
| --- | --- | --- |
| Missing stable model, five kinds, owner, lifecycle | `pkg/artifacts/types.go`; shared `testdata/claims.json`; `TestRoundTripClaims` retains duplicate IDs and stable identity through moved/edited provenance; every kind/lifecycle round-trips, owner remains optional | Accepted in final independent review |
| Missing applicability/path enforcement and no-read evidence | `validPath`, strict file/subtree claims and root subtree; `TestPathsAndEnums`; Linux inotify watches actual declared-file accesses, including unavailable and mode-000 input claims | Implemented; no observed access |
| Missing exact-span fixtures | `TestIndependentSourceCoordinates` independently checks literal UTF-8 bytes, full SHA-256, disjoint slices, CRLF, EOF with/without final LF, and expected coordinates; deliberately inconsistent unverified claims remain structurally accepted | Implemented; source verification stays deferred |
| Optionality/round trips and ordered duplicates | Pointer optionals, required arrays and booleans; `TestRoundTripClaims` and `TestInputDesignVariantsAndUnresolvedClaims` preserve all five reviewed input entries and independent pin/role/requiredness conflicts, duplicate edges and cycles | Implemented; no authority resolution |
| Missing malformed-input rejection | Bounded syntax scan, surrogate validation, exact-case closed object shape, integer spelling/range and semantic constraints; `TestStrictJSON`, nested missing/null/type cases, path/enum/input variants, Python schema negatives | Implemented; whole-document rejection |
| Missing resource evidence | Production reader/tokenizer/validator tests in `limits_test.go`; layered evidence below; every reader rejection helper requires nil catalog and diagnostic at most 256 bytes | Implemented; masked limits explicitly distinguished |
| Missing source-only compatibility evidence | `TestArtifactLikeMetadataDoesNotActivateCatalog` compares serialized IR and context before/after invalid artifact-like JSON files at three plausible names | Pass; no automatic activation |
| Artifact trust/non-execution missing | Byte-only public API; no filesystem or process package in production artifact code; Linux access watch and executable check-ID marker test | Pass on Linux; no source grounding or execution claim |

`docs/artifacts.schema.json` is a documented structural subset, not a replacement
for decoder checks. Runtime additionally enforces duplicate keys, Unicode escape
validity, integer spelling, UTF-8 byte lengths, envelope namespace equality, span
ordering, depth/bytes and combined counters. JSON Schema regex end anchors reject
terminal newlines. Existing closed IR/context contracts remain unchanged.

## Targeted checks and outcomes

All commands below ran from the repository root; no assertions or accepted
criteria were weakened. No failed targeted check was hidden or skipped.

- `go test ./pkg/artifacts ./pkg/agentctx`: exit 0, including the final additional
  EOF, mode-000 dependency and lower-counter fixtures.
- `go test -race ./pkg/artifacts ./pkg/agentctx`: exit 0 before the final test-only
  additions; the subsequent baseline will rerun the full race suite.
- `go test ./pkg/artifacts -run '^$' -fuzz FuzzDecode -fuzztime 10s -parallel 2`:
  exit 0, 77,617 executions, 110 newly interesting inputs; no crash, partial result,
  oversized diagnostic or unstable accepted round trip found. This bounded run
  is not an exhaustive proof.
- `/tmp/repoctx-m3-venv/bin/python -m unittest discover -s tests -p
  test_artifacts_schema.py -v`: exit 0, 4 tests; schema validity, shared positive
  declarations, required/closed fields, paths, types and enums checked.
- `go test ./pkg/artifacts -run
  'TestReaderResourceBoundaries|TestIsolatedProductionCountersAndMaskedReader|TestScalarBoundaries'
  -v`: exit 0. Verbose local log: `/tmp/repoctx-m3-resources.log`; durable test cases
  and summarized observations below are the reproducible evidence.
- Initial `git diff --cached --check`: flagged the deliberate CRLF in the exact
  source-byte fixture. Added a file-specific `.gitattributes` entry preserving
  raw bytes (`-text`) and recognizing CR-at-EOL while retaining default whitespace
  checks. The exact-byte/hash assertions were unchanged. Final staged whitespace
  check and local scope review passed before committing.

## Layered resource observations

All listed pairs passed their asserted expected acceptance/rejection. Counts are
inclusive. Zero API limits select ceilings; negative/above-ceiling limits fail.
Allocations based on wire content occur only after the 1 MiB byte guard. Syntax
scanning enforces depth before shape decoding, whose allocations remain bounded
by that input cap. No public structural-only bypass is exposed.

| Layer / configured limit | Accepted observation | Rejected observation / actual reason |
| --- | --- | --- |
| Reader bytes / 1,048,576 | Empty envelope padded to 1,048,576 bytes | 1,048,577 bytes: byte limit |
| Reader bytes / 90 | 90-byte envelope | 91 bytes: byte limit |
| Reader artifacts / 1024 | 1024, 296,025 bytes | 1025, 296,314 bytes: artifacts count |
| Reader applicability / 64 | 64, 2,297 bytes | 65, 2,327 bytes: applies_to count |
| Reader inputs / 64 | 64, 3,321 bytes | 65, 3,367 bytes: declared_inputs count |
| Reader artifact sources / 16 | 16, 3,183 bytes | 17, 3,370 bytes: sources count |
| Reader relationship sources / 16 | 16, 3,449 bytes | 17, 3,636 bytes: sources count |
| Reader relationships / caller 2 | 2 relationships | 3, 1,178 bytes: relationships count |
| Reader combined entries / caller 2 | One source plus one input | One source plus two inputs, 469 bytes: nested_entries count |
| Reader other counters / caller 1 | One artifact/applicability/input/source | Two of the selected kind: corresponding count |
| Reader depth / caller 5 then 4 | Same valid depth-5 catalog | At limit 4: depth limit |
| Isolated production validator / 4096 relationships | 4096 | 4097: relationships count |
| Isolated production validator / 16384 nested entries | 1024 artifacts with 16 sources each | Add one input: nested_entries count |
| Isolated production tokenizer / depth 16 | 16 open arrays | 17: depth limit |
| Reader masked relationships | No accepted hard-boundary claim | 4096 minimal edges, 1,093,721 bytes: byte limit |
| Reader masked aggregate | No accepted hard-boundary claim | 16,384 sources, 3,168,345 bytes: byte limit |
| Reader shape-invalid depth fixtures | No accepted-catalog claim | Depth 16, 33 bytes: expected object; depth 17, 35 bytes: depth limit |
| Reader scalar lengths | namespace 63, local ID 128, owner 256 UTF-8 bytes, check ID 256 ASCII bytes, path 4096 UTF-8 bytes | One more and empty values: corresponding grammar/length error |
| Reader coordinate range | 9,007,199,254,740,991 | One more in end byte/line/column: integer range; zero lines: positive-line rule |

Source/hash minimum, wrong digest spellings, forbidden paths, malformed optional
fields, bounds and integer spelling are covered separately from resource counters.
The 60,102-byte unknown-member diagnostic test does not echo its secret name/value.
Syntax errors identify container ordinals, while structural errors use trusted
field/index paths; neither includes arbitrary repository strings.

## Toolchains, dependencies and direct baseline step

Go: `go1.27.0-X:nodwarf5 linux/amd64`. C: GCC `16.2.1 20260810`.
Native parser dependencies compiled successfully in targeted tests. Default Python
is 3.14. Initially it lacked pip/build; this was a prerequisite gap, not a product
failure. Required packages were installed first in `/tmp/repoctx-m3-venv`, then
into the user's Python site-packages so direct `python3` works without activation.
Versions: build 1.6.1, jsonschema 4.26.0, pip 26.2.1, setuptools 84.0.0, wheel 0.48.0.
No system-owned package files or dependency requirements were changed.

Executed command, using the existing runner directly:

```sh
python3 scripts/m0_baseline.py --output /tmp/repoctx-m3-baseline.json
```

Before that tool step, this handoff explicitly recorded baseline results as
pending. The direct run subsequently passed **17/17 checks, zero failures**, exit
0; see the execution record below. The runner itself is unchanged. No external
blocker is currently established.

## Remaining work and retained limitations

M3-01 is accepted and has no remaining in-scope work. All M3 milestone
acceptance gates remain unchecked and unchanged. M3-02 duplicate/reference/conflict resolution,
M3-03 grounded adapters, M3-04 obligation selection/results and M3-05 dogfooding
remain out of scope. Exact-byte verification against caller-authorized files is
not performed by structural decoding. Namespace uniqueness and historical ID
non-reuse cannot be proved here. Lower caller limits do not grant authority.

Prior M0–M2 evidence remains historical: M0 recorded 17 passing commands; M1/M2
replay reports recorded 10/10 answerable tasks and 15/15 spans within budgets.
Those small retrieval fixtures do not establish agent outcomes, performance or
generalization. M2 verified-local mode is not an atomic filesystem snapshot and
portable race resistance remains limited. Earlier documentation discrepancies in
lower `docs/IR.md` sections remain unrelated, unedited work; current contracts
and code take precedence. No pre-existing external blocker was supplied or lost.

## Final review and retained baseline

The [final independent review](m3-01-final-review.md) accepts the whole M3-01
item and supports every [finding disposition](#finding-dispositions) above.
The [accepted design and design-review record](../design/m3-01-artifact-schema.md)
records acceptance of design revision
[`252686b67d2870882d4488469a019e055b125758`](https://github.com/tdeshazo/repoctx/commit/252686b67d2870882d4488469a019e055b125758).

Revision provenance:

- Implementation and final test additions:
  [`eedacb52f2d0faa60224e8aabc5cbc9f03cac816`](https://github.com/tdeshazo/repoctx/commit/eedacb52f2d0faa60224e8aabc5cbc9f03cac816),
  Git tree `225e80a2662bec47d0bce1ccbc8ed643b49d8586`.
- Tested and independently reviewed HEAD:
  [`35d91867426323d05f37b285bebac784b427a190`](https://github.com/tdeshazo/repoctx/commit/35d91867426323d05f37b285bebac784b427a190).
  Its changes after implementation contain only evidence documentation/reports.
- This acceptance-record commit is subsequent documentation-only bookkeeping.
  It does not claim a new execution of the baseline against its own revision.

The fresh command `python3 scripts/m0_baseline.py --output
artifacts/m3-01-baseline.json` passed **17/17 checks**, all exit codes zero.
The [machine-readable report](m3-01-baseline.json) is retained byte-for-byte;
its original revision, digest, environment, results and `report` path are intact.
It supersedes the earlier run described in the historical targeted-check and
baseline-step sections above. The earlier report remains available in Git at
`35d91867426323d05f37b285bebac784b427a190:docs/reports/m3-01-baseline.json`.

Checks include full Go tests and race tests, vet, both builds,
compile/context/schema checks, packaging, installation and launcher checks.
All four artifact-schema tests passed within baseline discovery. The final
review also records an independent rerun of `python3 -B -m unittest discover
-s tests -p test_artifacts_schema.py -v`: **4/4 passed**, and a passing
`git diff --check`. Thus the current baseline covers all final test additions,
including the race suite. Historical targeted runs are not relabeled as new runs.

Targeted contract evidence:

- [Types](../../pkg/artifacts/types.go), [strict decoder](../../pkg/artifacts/decode.go),
  [validator](../../pkg/artifacts/validate.go), and [schema](../artifacts.schema.json).
- [Contract and exact-coordinate tests](../../pkg/artifacts/decode_test.go),
  [shared five-kind claims](../../pkg/artifacts/testdata/claims.json), and
  [exact source bytes](../../pkg/artifacts/testdata/source.txt).
- [Production resource boundary tests](../../pkg/artifacts/limits_test.go) and
  [layered observations](#layered-resource-observations), including masked limits.
- [Linux no-read/non-execution tests](../../pkg/artifacts/io_linux_test.go),
  [source-only compatibility tests](../../pkg/agentctx/artifacts_test.go), and
  [dedicated artifact-schema tests](../../tests/test_artifacts_schema.py).

The report records the two pre-existing untracked workflow files and
`clean_checkout: false`. Its source digest is
`2f832ff804781cf76ee4c857ed88bfb50f6e283aa60ebc572daf7ba3d857d60d`.
The final reviewer reconstructed that pre-packaging digest and reported that
the fresh output preceded that review invocation's stdin file by about 13 ms.
These are reviewer observations, not a new freshness claim for this commit.

**Digest limitation:** the unchanged runner excludes every directory named
`artifacts`, including `pkg/artifacts`; its digest alone is insufficient provenance.
The final reviewer independently verified all ten tracked package files
byte-for-byte against HEAD and confirmed implementation, schema and tests were
unchanged since the implementation commit. Use those Git revisions together
with the baseline and targeted evidence. Platform-specific observations remain
limited to the recorded Linux environment; a bounded fuzz run is not exhaustive.
Structural validity grants no effective authority and does not verify source
claims, execute checks, or complete M3.

At handoff, the untracked baseline output, packaging-generated `build/` and
`src/repoctx.egg-info/`, and two pre-existing workflow files remain outside this
commit. No implementation, test, schema or baseline-runner file is changed.
M3-02–M3-05 and all M3 acceptance gates remain deferred and unchanged.
