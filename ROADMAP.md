# repoctx roadmap

> **Direction:** Make repoctx a dependable, source-linked repository evidence
> compiler: retrieve sufficient evidence for a task, explain its selection,
> preserve its provenance, and expose its limitations without taking over the
> agent's policy or execution environment.

- **Status:** Proposed development plan; not a release or delivery commitment.
- **Planning date:** 2026-09-08.
- **Baseline:** `6c303125e2a7864f944dc7e83716518fb3bb24a6`.
- **Version baseline:** `repoctx.ir/v1alpha3` and `repoctx.context/v1alpha1`.
- **Completion rule:** A milestone is complete only when its acceptance gates
  have linked, reproducible evidence. All work below is initially unchecked.
- **Scope of evidence:** Based on source and documentation inspection. This
  roadmap does not certify a new execution of the baseline test suite.

## 1. Product boundary

repoctx should compile repository source into reusable machine representations
and bounded agent-facing evidence. Its next step is **better evidence
sufficiency**, not simply smaller indexes or more language grammars.

The longer-term target is to connect source evidence to explicitly declared
components, decisions, contracts, and verification obligations. These remain
repository claims until a trusted application resolves their applicability and
authority. A parser must not promote repository prose into instructions.

In the research architecture, the source index and optional artifact model supply
part of the canonical repository context IR (R-CIR); task selection produces the
prompt/context IR (PIR). Execution capabilities (EIR) and software-result
verification (OIR) remain harness-owned. repoctx may describe obligations and
consume externally produced evidence without granting permissions or executing
repository commands.

### Preserve these invariants

1. **Separate index from context.** Compact AST/string/graph storage is for
   programs; readable evidence bundles are for agents. Storage compression is
   not prompt compression.
2. **Exact evidence, explicit incompleteness.** Preserve byte-exact source and
   source maps. Mark excerpts, missing capabilities, omitted candidates, and
   unresolved relationships. Never invent a summary to hide missing evidence.
3. **Budget the delivered representation.** Enforce limits on final serialized
   output. Exact token limits require the target tokenizer; the harness budgets
   wrappers, instructions, tools, history, and response capacity separately.
4. **Keep trust external.** Repository content, manifests, and declared commands
   cannot authorize themselves. Caller policy controls roots, scope,
   capabilities, approvals, and execution.
5. **Distinguish evidence from proof.** Syntax is not type resolution, an edge is
   not runtime causality, a digest is not authentication, and a valid bundle is
   not a passing software change.
6. **Measure before claiming improvement.** Report retrieval quality, agent use
   of evidence, software outcomes, and efficiency separately.

### Non-goals for this roadmap

- Building an autonomous coding agent, general workflow engine, or policy
  enforcement service inside repoctx.
- Running repository-provided build, test, plugin, or shell commands during
  ordinary compilation or retrieval.
- Claiming complete runtime semantics across all supported languages.
- Requiring embeddings, a model API, a vector database, or a long-running server
  for the core local workflow.
- Treating generated vendor instruction files, opaque latent representations,
  or automatically accumulated agent memories as canonical repository truth.

## 2. Starting point

The baseline already contains the following mechanisms; they are foundations to
preserve, not new roadmap deliverables.

| Area | Existing mechanism | Boundary to address |
| --- | --- | --- |
| Source index | Go and Tree-sitter frontends, normalized ASTs, symbols, occurrence records, forward/reverse CSR graph | Syntax-derived representation, not a complete repository-intent or dependency model |
| Context selection | Explicit symbols or lexical seeds followed by bounded graph expansion | Lexical matching uses names, semantic IDs, and paths rather than full source/document content |
| Evidence | Exact source, file hashes, physical byte coordinates, overlap merging, selection reasons, omission records | Markdown heading symbols do not supply their surrounding section prose |
| Freshness | Index digest and verification of every permitted indexed source file | New files and unindexed configuration inputs are outside that verification |
| Integration | CLI, public Go packages, Python launcher, model-neutral adapter, and vendored skill | No execution-plan or software-verification engine |
| Validation | Contract/regression tests and a recorded validation report | Fresh baseline runs and broader outcome evaluation must be reproducible |

Baseline references: [README][readme], [IR contract][ir-contract],
[handoff contract][handoff], [selector][selector], and
[validation record][validation].

The [Mothership comparison][comparison] is an exploratory evidence-handoff
experiment, not proof of autonomous retrieval or coding improvement. Its
assisted condition added manual answer-bearing excerpts because selected heading
spans were insufficient. Preserve that limitation when citing its results.

## 3. Milestone order

Milestone numbers express dependency and priority, not calendar estimates. They
are not package version numbers. Establish the baseline first; evidence
sufficiency and input completeness can then proceed in parallel. Evaluation
infrastructure starts in M0 rather than waiting until optimization.

| ID | Horizon | Outcome | Prerequisites | Initial status |
| --- | --- | --- | --- | --- |
| M0 | Now | Reproducible contracts, fixtures, and baseline measurements | None | Proposed |
| M1 | Now | Answer-bearing document and source retrieval | M0 | Proposed |
| M2 | Now, parallel with M1 | Complete, scope-aware compilation-input identity | M0 | Proposed |
| M3 | Next | Optional declared repository artifacts and grounded relationships | M1 and M2 | Proposed |
| M4 | Next | Controlled retrieval and agent-outcome evaluation | M1 and M2; M3 for artifact-specific claims | Proposed |
| M5 | Later, evidence-gated | Incremental compilation and efficient repeated serving | M2 and a measured M4 bottleneck | Proposed |
| M6 | Release gate | Stable contracts, supported distribution, and maintainability | Relevant preceding gates for each shipped capability | Proposed |

A release may ship a completed subset. Untested optional providers and service
features must remain experimental or disabled. Stable protocol behavior does
not, by itself, justify a performance or task-success claim.

## 4. M0 — Establish reproducible contracts and evaluation fixtures

**Objective:** Make correctness and improvement observable before expanding the
feature set.

**Primary surfaces:** Existing tests in `pkg/agentctx`, `pkg/compiler`, `pkg/ir`,
`internal/lang`, and `internal/sourceroot`; proposed CI and `evals/` fixtures.

### Work

- [ ] **M0-01 — Record a fresh baseline.** Automate Go tests, race tests, vet,
  both CLI builds, IR/context schema checks, and Python distribution smoke
  tests. Record the source commit, Go/C/Python toolchains where used, platform,
  grammar dependencies, and commands. Do not treat historical coverage numbers
  as current results.
- [ ] **M0-02 — Create a small deterministic evaluation corpus.** Start with at
  least 12 distinct fixtures: four documentation tasks, four code tasks, two
  mixed code/document tasks, and two no-answer or access-restricted tasks.
  Store tasks, answer-bearing spans, expected relationships, allowed scope,
  budgets, and scoring rules separately from agent-visible inputs. This count
  is an initial engineering target, not a statistically sufficient benchmark.
- [ ] **M0-03 — Reproduce source-inspection findings.** Add focused tests for
  import relationship occurrence sites and the scope/freshness treatment of
  `go.mod`. The import-site issue is a hypothesis from static review: reproduce
  it before documenting it as a confirmed bug, then fix it or record why it
  does not reproduce. Test both repository-module and external imports.
- [ ] **M0-04 — Make claims traceable.** Update validation and comparison
  documentation with source revisions, exact conditions, limitations, and
  reproducible artifacts where disclosure is permitted. Keep private source
  and account configuration out of fixtures and published logs.

### Acceptance gates

- [ ] The documented baseline commands run from a clean checkout in a declared
  environment, with machine-readable results and failures retained.
- [ ] Contract checks cover exact bytes, UTF-8/CRLF positions, final JSON and
  Markdown payload bounds, tokenizer-hook errors, overlap merging, stale
  sources, denied paths, and malformed indexes.
- [ ] Each fixture has an independently inspectable expected result. Repeating
  one task does not increase the reported number of distinct tasks.
- [ ] Test coverage, artifact validity, retrieval quality, and agent outcomes are
  reported as different measurements.

## 5. M1 — Retrieve sufficient document and source evidence

**Objective:** A bounded bundle should contain the facts needed for its intended
task, not merely the heading or symbol nearest those facts.

**Primary surfaces:** `internal/lang/treeast`, `pkg/ir`, `pkg/agentctx`, and the
context schema. Introduce retrieval units without silently changing existing
symbol-span meanings.

### Work

- [ ] **M1-01 — Add document retrieval units.** Represent documents, sections,
  paragraphs, checklist items, and tables with exact spans and containment.
  Keep a heading span separate from its section-content span. Define section
  boundaries at the next heading of equal or higher level, including EOF and
  nested sections. Handle repeated headings and documents without headings.
- [ ] **M1-02 — Add body-text lexical retrieval.** Search bounded source and
  document blocks, including comments and error literals, alongside existing
  name/path matching. Use a documented deterministic ranking baseline before
  adding embeddings. Report the matched fields and ranking components. Apply
  exclusions before building text indexes; protect their contents and caches as
  source-derived sensitive artifacts, not as sanitized metadata.
- [ ] **M1-03 — Select task-relevant excerpts.** Prefer answer-bearing blocks and
  query-centered source windows over declaration prefixes alone. Preserve
  signatures or necessary local context where feasible. Noncontiguous evidence
  must remain separate exact spans; omissions must not look like continuous
  source or a complete behavioral description.
- [ ] **M1-04 — Preserve a transparent retrieval contract.** Support explicit
  unit references in addition to symbols. Explain exclusions, budget tradeoffs,
  unavailable context, and no-match results. Keep required-seed failure explicit;
  do not substitute unrelated content. Keep retrieval scores distinct from
  confidence or sufficiency guarantees.
- [ ] **M1-05 — Expand regression coverage.** Cover nested/repeated headings,
  lists, tables, Unicode, long sections, errors appearing only in function
  bodies, and declarations whose decisive branch occurs near the end. Fenced
  code remains source evidence, never recursively promoted into executable
  language semantics merely because of its fence label.

### Acceptance gates

- [ ] A public or permission-cleared fixture equivalent to the Mothership
  question is answerable from an unmodified repoctx bundle, using the original
  question without the correct feature name or manually supplied answer prose.
- [ ] That fixture includes both the priority/completion evidence and the exact
  next unmet criterion, with byte-accurate source references.
- [ ] A literal or fact found only in a body paragraph or function body can seed
  retrieval even when names and paths do not match the question.
- [ ] JSON and Markdown deliver equivalent selected evidence and omission
  semantics within their respective final-output budgets.
- [ ] Relevant content that cannot fit is explicitly excerpted or omitted; the
  output never implies exhaustive coverage or guaranteed answerability.

**Release artifact:** Updated schemas, migration notes for changed contracts,
replayable fixtures, and before/after retrieval measurements.

## 6. M2 — Make snapshot identity cover every compilation input

**Objective:** Detect relevant repository changes and configuration drift without
confusing a source hash with a complete, authenticated snapshot.

**Primary surfaces:** `pkg/compiler`, `pkg/ir`, `pkg/agentctx/source.go`, and
`internal/sourceroot`.

### Work

- [ ] **M2-01 — Add a compilation-input manifest.** Record the permitted source
  inventory, hashes, configuration inputs, compiler/frontend identities, and
  effective selection/build profiles. Include inputs such as `go.mod` when they
  affect graph construction. Record relevant negative dependencies, such as an
  absent optional config file whose later creation would change compilation.
- [ ] **M2-02 — Enforce scope on all inputs.** Apply caller-controlled policy to
  auxiliary configuration reads as well as indexed source. When a needed input
  is denied, fail or publish the affected capability as unavailable; do not read
  it silently. Specify whether ignore files are honored, which defaults apply,
  and what additions lie inside the discovery boundary.
- [ ] **M2-03 — Separate identities.** Distinguish source snapshot, index schema,
  compiler/provider profile, and task-bundle identity. A task cache key must also
  include query/explicit units, scope, selection settings, renderer version, and
  budget/tokenizer identity where applicable. Exclude host paths and timestamps
  from reproducibility-critical content identities.
- [ ] **M2-04 — Define consistency modes.** Retain verified local reads; support
  immutable, caller-controlled snapshots for reuse. Git revisions may identify
  provenance, but dirty trees, untracked permitted files, configuration, and
  external provider inputs need explicit treatment. Reject incomplete or mixed
  generations and publish manifests atomically.
- [ ] **M2-05 — Bound resource and trust exposure.** Add regression tests for
  auxiliary-input scope, source inventory changes, path races within the stated
  platform threat model, malformed data, and total compilation/read limits.
  Do not advertise portable race resistance beyond tested guarantees.

### Acceptance gates

- [ ] Adding, deleting, renaming, or modifying a discoverable permitted source
  invalidates the appropriate snapshot. Changes outside declared scope have
  documented behavior rather than an implied whole-repository guarantee.
- [ ] Modifying or introducing a graph-affecting configuration input invalidates
  the index or its affected view before serving it as current.
- [ ] Every byte that affects derived meaning is either a declared input or an
  explicitly unavailable dependency. Denied inputs never become a hidden bypass.
- [ ] Identical inputs and compiler profiles reproduce identical canonical
  identities after relocation. A rejected update leaves the prior complete
  generation usable, not a mixture of old and new data.
- [ ] No cache or snapshot reuse crosses incompatible authorization scopes.
  Denied content, names, paths, and derived metadata remain protected according
  to the declared disclosure policy.

## 7. M3 — Connect evidence to declared repository intent

**Objective:** Model the repository knowledge that syntax alone cannot provide,
without turning repoctx into a policy authority or an execution engine.

**Primary surfaces:** A proposed optional artifact/manifest model, compiler
adapters, relationship provenance, and the context handoff. Define the public
contracts before committing to package names or a manifest filename.

### Work

- [ ] **M3-01 — Define a minimal optional artifact schema.** Support stable IDs,
  artifact kind, owner, applicability, lifecycle, source spans, declared inputs,
  and relationships. Start with components, decisions, contracts, requirements,
  and verification obligations. Preserve the ordinary source-only workflow.
- [ ] **M3-02 — Separate declarations from effective authority.** Repository
  metadata may claim normative status, scope, or supersession. Only trusted
  caller configuration determines whether those claims apply. Detect duplicate
  IDs, broken references, invalid scopes, supersession cycles, and conflicting
  accepted requirements; never resolve normative conflicts by file order alone.
- [ ] **M3-03 — Add grounded relationship adapters.** Resolve repository-local
  document links and pilot one demand-driven language/build provider. Distinguish
  containment, references, declared ownership, and provider-resolved dependencies
  from heuristic calls. Include provider version, input identity, evidence sites,
  resolution method, and coverage limits. Test import aliases and unresolved
  targets without claiming universal dispatch resolution.
- [ ] **M3-04 — Emit obligations, not execution.** Select applicable contract and
  check identifiers alongside source evidence. Link requirements to supporting
  tests as declared or observed relationships, not proof of execution. A separate
  runner may return results bound to commit, environment, command/check identity,
  exit status, and artifacts; repoctx must not manufacture a passing status.
- [ ] **M3-05 — Dogfood the model.** Describe repoctx's own packages, contracts,
  development commands, and roadmap requirements using the proposed artifacts.
  Verify links mechanically and retrieve an incomplete milestone plus its next
  unmet gate without answer-informed prompts or duplicate instruction manuals.

### Minimum artifact sketch — design only

```yaml
# Illustrative shape, not a supported schema or current CLI configuration.
artifacts:
  - id: component.context-builder
    kind: component
    owner: maintainers
    applies_to: ["pkg/agentctx/**"]
    source: docs/AGENT_CONTEXT.md
    status: active
  - id: requirement.exact-evidence
    kind: requirement
    applies_to: ["pkg/agentctx/**"]
    source: docs/AGENT_CONTEXT.md
    status: active
relationships:
  - from: component.context-builder
    kind: governed_by
    to: requirement.exact-evidence
verification_obligations:
  - id: check.context-contract
    requirement: requirement.exact-evidence
    runner_check_id: context-contract-tests
# The trusted caller resolves scope, authority, and runner_check_id.
# No field here grants filesystem, shell, network, or tool permission.
```

### Acceptance gates

- [ ] A component-level task retrieves relevant code, the applicable declared
  requirement/contract, and verification obligations with source provenance.
- [ ] Superseded or conflicting artifacts are diagnosed rather than silently
  treated as current. Historical material remains retrievable as history.
- [ ] Pull-request-head manifests and repository text cannot increase their own
  authority, widen caller scope, activate an executable plugin, or launch a check.
- [ ] Each provider advertises actual coverage and fails explicitly on unsupported
  inputs. Declared, syntactic, heuristic, and provider-resolved edges are separable.
- [ ] Source-only repositories still compile and serve without mandatory catalog
  metadata, model services, or a build environment.

## 8. M4 — Demonstrate retrieval quality and downstream outcomes

**Objective:** Establish which benefits come from evidence selection, graph
structure, declared artifacts, or reduced tool use.

**Prerequisite distinction:** Core retrieval evaluation needs M1/M2. Claims about
repository-intent features need M3. Evaluation tooling and fixtures begin in M0.

### Work

- [ ] **M4-01 — Build a held-out task set.** Use an initial target of at least 30
  distinct answerable tasks across at least three permission-cleared repositories,
  plus no-answer, contradictory-evidence, and denied-scope cases. Include
  documentation lookup, code localization, cross-file analysis, and changes with
  executable checks. This is a pilot floor, not evidence of broad statistical
  generalization. Keep tuning tasks separate.
- [ ] **M4-02 — Compare appropriate conditions.** Include ordinary repository
  tools, bounded lexical snippets, repoctx without graph expansion, and repoctx
  with graph expansion. Add artifact-aware and human-curated oracle conditions
  separately when relevant. Do not attribute manual curation to repoctx.
- [ ] **M4-03 — Control experiments.** Pin commits, compiler/provider profiles,
  model/harness configuration, permissions, budgets, and scoring rules. Use fresh
  workspaces for change tasks; randomize or interleave trial order and record
  cache state. Give matched end-to-end conditions the same tool permissions.
  Run evidence-only tests as a separate track with the same no-tools constraint
  for all compared evidence suppliers.
- [ ] **M4-04 — Publish complete accounting.** Capture compilation, update,
  retrieval, agent, and verification costs separately. Report cold and warm
  workloads, all permitted tool calls, failures, timeouts, abstentions, and raw
  provider usage fields. Do not double-count cached input or reasoning tokens
  that are subsets of other usage fields.
- [ ] **M4-05 — Predeclare decision rules.** Choose primary metrics, quality
  tolerances, uncertainty reporting, and practical improvement thresholds before
  evaluating the held-out set. Treat an underpowered result as inconclusive,
  not as proof of equivalence or non-regression.

### Required measurements

| Question | Measures |
| --- | --- |
| Was relevant evidence found? | Answer-bearing span recall, selected-evidence precision, required-artifact coverage, first relevant result |
| Was evidence preserved? | Exact-span fidelity, completeness/excerpt accuracy, required-condition retention under budget pressure |
| Did the agent use it correctly? | Source-supported claims, citation accuracy, unsupported conclusions, follow-up retrieval, incorrect abstentions |
| Did the task succeed? | Answer correctness or verified patch outcome, regressions, required checks, unresolved obligations |
| Was the system efficient? | End-to-end time, p50/p95 retrieval latency, model usage, tool calls, cold/warm compilation cost, memory |
| Was it safe within its stated scope? | Unauthorized evidence served, scope-crossing cache reuse, instruction-promotion failures, resource-limit behavior |

### Acceptance gates

- [ ] A benchmark version, task inventory, configurations, scoring code, and
  permission-cleared traces make the results reproducible.
- [ ] No hidden answer terms, manually appended answer prose, or oracle file
  paths enter a condition presented as autonomous retrieval.
- [ ] Benefits are stated only for tested workloads. Claims of lower cost retain
  their declared quality guardrails; faster incorrect answers are not a win.
- [ ] Software-change claims use independent executable checks or documented
  review criteria. Read-only question answering is not relabeled coding success.
- [ ] The report records uncertainty, failure categories, cache effects, and
  unmeasured costs. An inconclusive result keeps the claim experimental.

## 9. M5 — Amortize verified compilation and serving

**Objective:** Reduce repeated work only after profiling identifies its cost and
M2 provides a sound invalidation model.

**Entry gate:** Publish a representative bottleneck profile and a predeclared
performance target. Do not build a daemon merely to complete this milestone.

### Work

- [ ] **M5-01 — Profile the complete path.** Measure discovery, parsing, linking,
  validation, serialization, source verification, ranking, and materialization
  across repository sizes. Attribute time, memory, and artifact growth.
- [ ] **M5-02 — Add content-addressed incremental artifacts.** Cache per-file
  parsing and dependent views under complete input/profile identities. Handle
  reverse dependencies, deleted files, and configuration changes. Global dense
  graph IDs remain snapshot-local even when individual fragments are reused.
- [ ] **M5-03 — Reuse verified immutable sources.** Avoid rereading every source
  on every query only when the caller provides an enforceable immutable snapshot
  or equivalent validated generation. Metadata timestamps alone are not proof of
  content identity. Keep strict local verification available.
- [ ] **M5-04 — Add serving only where justified.** A library cache or optional
  service must support bounded memory, eviction, cancellation, atomic generation
  swaps, and authorization-scoped identities. A daemon may be deferred if a
  simpler local design meets the measured need.

### Acceptance gates

- [ ] Incremental and clean builds produce identical canonical semantic outputs
  for the same inputs/profile, including add/delete/rename/configuration tests.
- [ ] Eviction, restart, cancellation, and interrupted publication cannot expose
  mixed generations or evidence from another caller's scope.
- [ ] The measured target is met on the declared workload without violating
  evidence fidelity, freshness, resource, or quality gates. Publish warm and cold
  results rather than only the favorable case.
- [ ] The local CLI remains usable without a service or external database.

## 10. M6 — Stabilize contracts and distribution

**Objective:** Make supported behavior predictable for users and integrators.

This gate applies to the capabilities actually shipped. It does not require
implementing every optional provider or a server, and it does not require a
positive agent-performance claim.

### Work

- [ ] **M6-01 — Define compatibility policy.** Version the Go API, repository IR,
  agent context, and optional artifact/provider contracts explicitly. Specify
  supported readers, unknown-field handling, migrations, deprecations, and when
  recompilation is required. Do not silently redefine existing IDs, spans,
  completeness labels, or trust semantics.
- [ ] **M6-02 — Maintain conformance fixtures.** Test old supported indexes,
  unsupported-version errors, JSON/Markdown projections, tokenizer callbacks,
  progressive disclosure, and provider capability negotiation. Preserve
  application-owned roots, policies, and payload caps across adapters.
- [ ] **M6-03 — Validate distribution.** Test source builds, both Go entry
  points, Python wheel/sdist installation, launcher behavior, and clean-machine
  operation for each advertised platform. Document supported toolchains and CGO
  requirements; do not imply untested platform or wheel portability.
- [ ] **M6-04 — Establish release maintenance.** Publish changelogs, artifact
  checksums/provenance, license and dependency notices, a security-reporting
  process, and contributor guidance. Keep skill instructions, examples, schemas,
  validation reports, and claims synchronized with released behavior.

### Acceptance gates

- [ ] Clean-install tests pass for every advertised distribution/platform pair.
- [ ] Users can identify the executable, schema, provider, and source generation
  behind an output and determine whether an older index is compatible.
- [ ] Documentation clearly separates supported, experimental, unsupported, and
  externally enforced behavior.
- [ ] Every release claim links to a test, fixture, evaluation report, or stated
  limitation appropriate to that claim.

## 11. Cross-cutting release gates

These apply throughout the roadmap, not only at M6.

| Gate | Required evidence |
| --- | --- |
| Source fidelity | Exact byte ranges and hashes survive selection, coalescing, rendering, and expansion |
| Budget integrity | Final serialized payload stays within its cap; tokenizer failures and impossible budgets fail explicitly |
| Scope and trust | All input paths and derivatives respect caller policy; repository data cannot grant authority or permissions |
| Reproducibility | Same semantic inputs/profile yield the same canonical output; concurrent mutations are rejected or isolated by the declared mode |
| Semantic honesty | Heuristics, unresolved targets, parse failures, unsupported providers, and omitted evidence stay visible |
| Security/resource bounds | Negative tests, fuzzing where useful, adversarial repository fixtures, and documented platform limits |
| Compatibility | Changed wire meanings receive an explicit version/migration decision and conformance coverage |
| Outcome claims | Correctness, cost, latency, and software success are independently supported rather than inferred from index size |

## 12. Deferred directions

The following require an evaluation-backed proposal rather than automatic
inclusion in the core:

- **Embeddings and learned ranking:** after body-text lexical and graph baselines
  expose a persistent semantic-retrieval gap.
- **Additional grammars or type-aware providers:** when a target workload
  justifies the coverage and maintenance cost.
- **Vendor-specific adapters or MCP transport:** when integrations require them;
  they must preserve the same evidence and trust contracts.
- **Generated agent instruction projections:** only from separately approved
  policy sources, never by promoting retrieved repository text into instructions.
- **Learned summaries, latent context, or KV artifacts:** after sufficient,
  source-linked textual evidence and model-specific evaluation exist.
- **Automated guidance tuning:** only with held-out evaluation, reviewed changes,
  and no silent self-modification of authoritative repository artifacts.

## 13. Execution and maintenance

Start with three small implementation tracks: baseline/fixtures and reproduction
of the suspected import-site issue; document units plus body-text retrieval; and
complete compilation-input accounting. Keep M1 and M2 independently reviewable
while coordinating their schema changes. Do not combine all milestones into one
IR rewrite.

Use task IDs such as `M1-02` in issues and pull requests. Before implementation,
assign an owner, record prerequisites, and state the intended acceptance evidence.
New schema, trust-boundary, provider, or cache-identity decisions need a short
reviewed design record.

To complete a checklist item, attach the implementing revision and its tests or
report. To complete a milestone, satisfy every gate or explicitly revise its
scope through review. Do not convert a skipped gate into an implied success.
Update this file when dependencies or priorities change, and retain the rationale
for deferred work.

**Definition of progress:** More tasks can be answered or changed correctly from
traceable, bounded evidence, with complete input identity and visible limitations.
More indexed bytes, more graph edges, or fewer tokens alone do not establish it.

## Baseline source references

These links are repository-relative for use at the repository root. The baseline
commit above identifies the versions reviewed; linked files will evolve as the
roadmap is implemented.

- [README and current product boundary][readme]
- [Repository IR contract][ir-contract]
- [Agent context handoff contract][handoff]
- [Recorded validation and its scope][validation]
- [Exploratory Mothership comparison][comparison]
- [Current selector][selector]
- [Current context builder][builder]
- [Current relationship materialization][relationships]
- [Compiler and auxiliary-input handling][compiler]
- [Source verification][source]
- [Vendored agent skill][skill]

[readme]: README.md
[ir-contract]: docs/IR.md
[handoff]: docs/AGENT_CONTEXT.md
[validation]: docs/VALIDATION.md
[comparison]: docs/reports/mothership-repoctx-codex-exec-comparison.md
[selector]: pkg/agentctx/select.go
[builder]: pkg/agentctx/build.go
[relationships]: pkg/agentctx/relationships.go
[compiler]: pkg/compiler/compiler.go
[source]: pkg/agentctx/source.go
[skill]: skills/repoctx/SKILL.md
