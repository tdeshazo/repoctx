# M3-01 evidence and continuation handoff

Date: 2026-09-18. Iteration: 1 of 12.
Owner: this workflow's implementation role.
Status: **DESIGN_REWORK revised — awaiting next iteration's independent design review.**
This is readiness for the next M3-01 step, not implementation completion or
independent review approval. No code, schema implementation or roadmap checkbox
changes are authorized in this iteration. No blocker prevents design review.

## Exact scope

The selected item in [ROADMAP.md](../../ROADMAP.md) remains unchecked:

> **M3-01 — Define a minimal optional artifact schema.** Support stable IDs,
> artifact kind, owner, applicability, lifecycle, source spans, declared inputs,
> and relationships. Start with components, decisions, contracts, requirements,
> and verification obligations. Preserve the ordinary source-only workflow.

Read M3's work and acceptance gates, the cross-cutting gates in section 11 and
execution rules in section 13. M3-02 through M3-05 remain out of scope, including
authority resolution, grounded adapters, obligation retrieval and dogfooding.
The roadmap YAML is illustrative, not an existing parser/API contract.

## Baseline and pre-existing work

Inspection baseline: `69df7f840fa1c74212166bc64302c1b8c5e55333`
(`feat(snapshot)!: implement M2 compilation-input consistency`). This differs
from the roadmap's original planning baseline
`6c303125e2a7864f944dc7e83716518fb3bb24a6`.

Initial `git status --porcelain=v1 --untracked-files=all`:

```text
?? docs/workflows/next-roadmap-item.md
?? next-roadmap-item.yaml
```

There were no tracked modifications. Preserve both pre-existing untracked files;
neither is an instruction to expand this task. Neither this report nor
`docs/design/m3-01-artifact-schema.md` existed, so there was no unfinished M3-01
artifact to recover. Recent commits also include `ef2a40c` (M1), `6cee74f`
(discovery), `e915946` (M0) and `f6bf293` (roadmap).

No AGENTS.md was found in the repository (including hidden paths) or along the
workspace's ancestor chain through `/`. Read the `golang-how-to` routing skill
and applied `golang-documentation` to this bounded design-document task. No Go
code or exported Go names are being introduced.

## Prerequisite evidence inspected

The roadmap records M0–M2 work and acceptance gates as complete. The following
retained artifacts support proceeding with design. They are historical evidence;
this iteration does not certify a fresh baseline or independently rerun replays.

| Prerequisite | Evidence inspected | Result and limitation |
| --- | --- | --- |
| M0 | [baseline contract](../M0_BASELINE.md), [JSON report](m0-baseline.json) | Report has 17/17 passing commands, zero failures, clean temporary revision `3a0b7378c5ab9ae7f05a41c04eef081e281f06cc`, toolchains/platform and tree digest. Its original fixture inventory predates the current v2 corpus. |
| M1 | [replay](../../evals/runs/m1-retrieval-20260910.json), [tests](../../pkg/agentctx/m1_test.go), [consolidated report](mothership-repoctx-codex-exec-comparison.md) | 12 distinct fixtures; after JSON and Markdown each cover 10/10 answerable tasks and 15/15 spans within budgets. Before: 4/10 tasks and 8/15 spans. Artifact records working-tree changes above `6cee74fe431e414d9768676da95331bb81ca4852`, not a clean final M1 revision. |
| M2 | [replay](../../evals/runs/m2-inputs-retrieval-20260910.json), [compiler tests](../../pkg/compiler/inputs_test.go), [context tests](../../pkg/agentctx/m2_test.go), consolidated report above | Both conditions retain 10/10 tasks and 15/15 spans in each format within budgets. Artifact records working-tree changes above `ef2a40cd0470c677e4d5908c1b4f5c77258b5b69`. Tests cover drift, scope, identities, bounded reads and publication; their presence is not a fresh pass. |

The replay JSON was parsed and only metadata/summaries printed; oversized trial
payloads were not dumped. Binary hashes and trial payloads remain in the original
artifacts. No agent-outcome, latency or generalization claim follows from these
small retrieval fixtures. M2 verified-local mode detects observed drift and is
not an atomic filesystem snapshot; portable race resistance remains limited.

## Current contracts and reconciliation

Inspected [IR documentation](../IR.md), [context contract](../AGENT_CONTEXT.md),
the current and historical JSON schemas, and these implementation surfaces:
`pkg/ir/ir.go`, `pkg/ir/inputs.go`, `pkg/compiler/compiler.go`,
`pkg/compiler/inputs.go`, `pkg/compiler/io.go`, `pkg/agentctx/types.go`,
`pkg/agentctx/build.go` and `pkg/agentctx/source.go`.

Current IR is `repoctx.ir/v1alpha4` with required compilation inputs; context is
`repoctx.context/v1alpha3` with source/profile/task identities. Discovery remains
`repoctx.discovery/v1alpha1`. All current top-level schemas reject extra fields.
Historical IR v1alpha3 and context v1alpha1/v1alpha2 schemas remain available.
`Build` explicitly requires IR v1alpha4. Compiler inputs currently account for
supported source inventory and root `go.mod`; they are not an arbitrary catalog
loader. Span coordinates are physical, byte-based and end-exclusive.

Known documentation discrepancy: lower sections of `docs/IR.md` still say new
files/configuration are not detected and describe serving version 3. Those
paragraphs predate M2 and conflict with its header, current schemas,
`agentctx.Build`, `compiler.LoadInputs` and the M2 context migration contract.
Use the latter for this design. No unrelated contract documentation was edited.
Historical M0 limitations likewise describe M0, not current M2 behavior.

## Plan and acceptance criteria

1. Establish the baseline, preserve existing work and confirm prerequisite
   records — done by source/report inspection above.
2. Define the additive public contract before implementation — drafted in the
   [design record](../design/m3-01-artifact-schema.md).
3. Check design completeness and handoff consistency — checked against the
   requested fields, trust boundaries, compatibility and test matrix.
4. Next iteration: review the design, settle implementation locations, implement
   only the M3-01 structural contract when authorized, and retain test evidence.
   Do not select another roadmap item on resume.

The design specifies stable namespaced IDs, five initial kinds, optional claimed
owner, explicit file/subtree applicability, lifecycle claims, exact source spans,
declared inputs and source-grounded relationship declarations. It specifies
required/optional fields, strict versions/unknown fields, malformed-input
handling, bounded decoding and source-only behavior. It separates structural
acceptance from source verification and later effective authority. No commands,
scope grants or verification-result fields are introduced.

Compatibility decision: standalone proposed `repoctx.artifacts/v1alpha1`; no
existing IR/context changes, migration, automatic manifest discovery, new cache
or catalog enrichment. Later integration must separately review input/generation
identity and wire versions. The design includes a positive/negative test matrix;
none of those future tests are represented as executed.

## Validation and remaining work

This iteration's checks: inspected Git status/history; parsed retained report
JSON and schema metadata; inspected relevant APIs/test cases; checked new-document
relative links, exact roadmap item wording, whitespace and unchanged tracked
files. No full Go/Python test suite was run for these two documentation additions.

Files created: this handoff and the design record only. The pre-existing workflow
files and ROADMAP.md remain unchanged. No new implementing revision exists yet.

Remaining evidence before checking M3-01: independent design review, implemented
schema/structural decoder, accepted and rejected fixtures, resource-limit and
source-only regression results, and an implementing revision. These are planned
work, not a current environmental blocker. M3-02 duplicate/reference/conflict
diagnostics and every other M3 item remain deferred.

## DESIGN_REWORK continuation (2026-09-18)

The supplied independent review requested two changes: define true/false
input dependency claims and duplicate-path wire behavior; replace impossible
reader-only resource boundary coverage with achievable layered evidence.
The design now defines necessary versus optional claimed dependencies, preserves
all repeated inputs (including conflicting pins, roles and requiredness), and
embeds a five-entry JSON design fixture plus positive/negative variants.
Resource evidence now separates reachable reader boundaries, lower caller-limit
pairs, isolated production validator/tokenizer pairs, and masked rejections.

Review disposition: revised for re-review, **not accepted**. Next iteration must
obtain independent design acceptance before implementing M3-01. No implementation,
schema, implementation test files, M3-02–M3-05 features, or checkbox changes were made.
Existing source-only regression and conformance requirements remain outstanding;
historical evidence and its limitations above are unchanged. No migration is
needed for this documentation revision; the proposed standalone version and
future integration restrictions remain unchanged.

At entry, both this report and the design were already untracked handoff files.
Their historical iteration-1 account above is retained. The other untracked files
were `docs/workflows/next-roadmap-item.md` and `next-roadmap-item.yaml`; preserve
them outside this commit. Baseline HEAD remains `69df7f840fa1c74212166bc64302c1b8c5e55333`.
No repository or ancestor AGENTS.md applies. Documentation and conventional-commit
skills apply to this bounded revision. No external blocker was found.

Validation commands, outcomes and revision are recorded below. Design JSON and
size calculations are documentation checks only, not implementation conformance.

### Revision checks

- `git status --porcelain=v1 --untracked-files=all` and
  `git log -1 --format='%H %s'`: exit 0; four untracked files, baseline above.
- Repository/ancestor instruction search and roadmap sections 7, 11, 13
  inspection: exit 0; no applicable AGENTS.md; M3-01 remains unchecked.
- Initial `python3 - <<'PY'` documentation check: exit 1 before Python ran;
  sandbox could not create the shell here-document temporary file. Retried with
  approved filesystem escalation: exit 0. Parsed five fixture entries, checked
  independent duplicate/conflict cases and pin lengths, verified all relative
  links and trailing whitespace, and reproduced 1,093,721 bytes. No persistent
  blocker or code change resulted.

Reproducible compact check (run from repository root; stdout is the artifact):

```python
import json, re
from pathlib import Path
p = Path("docs/design/m3-01-artifact-schema.md")
entries = json.loads(re.search(r"```json\n(.*?)\n```", p.read_text(), re.S)[1])
assert len(entries) == 5 and entries[0] == entries[1]
for index, field in [(2, "sha256"), (3, "role"), (4, "required")]:
    assert entries[index][field] != entries[0][field]
assert all(re.fullmatch("[0-9a-f]{64}", x["sha256"]) for x in entries)
span = dict(path="a", sha256="0" * 64, start_byte=0, end_byte=1,
            start_line=1, end_line=1, start_byte_column=0, end_byte_column=1)
edge = {"from": "a:a", "to": "a:a", "kind": "contains",
        "resolution": "declared", "sources": [span]}
wire = json.dumps(dict(version="repoctx.artifacts/v1alpha1", namespace="a",
                       artifacts=[], relationships=[edge] * 4096),
                  separators=(",", ":")).encode()
assert len(wire) == 1093721 and len(wire) > 1048576
for file in [p, Path("docs/reports/m3-01-evidence.md")]:
    text = file.read_text()
    assert text.endswith("\n")
    assert all(line == line.rstrip() for line in text.splitlines())
    for target in re.findall(r"\]\(([^)]+)\)", text):
        if "://" not in target:
            assert (file.parent / target.split("#")[0]).exists(), target
print("PASS: design fixtures, 1093721-byte catalog, relative links, whitespace")
```

Run the retained check with:
`python3 -c 'import pathlib,re; s=pathlib.Path("docs/reports/m3-01-evidence.md").read_text(); exec(re.search(r"```python\n(.*?)\n```",s,re.S)[1])'`.

No Go tests, schema conformance tests or source-only regressions were run:
implementation is explicitly deferred by DESIGN_REWORK. The two Markdown files
are the only revision artifacts. The design-only commit is identifiable with
`git log -1 --format=%H -- docs/design/m3-01-artifact-schema.md` immediately after
this handoff commit; it is not an implementing revision. An implementing revision
and independent design acceptance remain outstanding.

The retained compact check above ran with exit 0 and printed
`PASS: design fixtures, 1093721-byte catalog, relative links, whitespace`.
`git diff --cached --check` exited 0. The full staged diff was inspected with
`git diff --cached -- docs/design/m3-01-artifact-schema.md docs/reports/m3-01-evidence.md`
(exit 0); only these two documentation files were staged. Inspection corrected
the depth example to five containers: root, envelope array, record, source array,
source object. This depth is accepted at caller limit 5 and rejected at limit 4.
