# M6-07 first-use record

M6-07 remains open. This record preserves one six-trial first-use run against
the fixed source export at
`1dc9657c30c06ff26320fead0a23d28fbd947781`. It does not make a comparative,
efficiency, or generalization claim.

The frozen task inputs and evaluator key are under
[`evals/first-use`](../../evals/first-use/README.md). The source export excluded
evaluation, runner, historical-report, build, and repository-control material;
its file hashes and exclusions are in the archived
[`source-export-hashes.json`](../../evals/runs/m6-07-20260922/agent/source-export-hashes.json).

## Agent trials

Each task ran once with ordinary tools and once with ordinary tools plus the
skill from the pinned source revision and pinned `repoctx` binary. All six
completed with exit code 0, no timeout, unchanged source, and the 24-call
advisory cap respected.
The run used `gpt-5.6-luna` at medium reasoning effort, with a 300-second
per-trial timeout. Raw runtime and usage are retained in the six archived
records; they are not normalized or used as an outcome metric.

| Condition | Trials | Wall seconds | Tool items | Input | Output | Input + output | Cached input | Reasoning output |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ordinary tools | 3 | 174.95 | 22 | 816,488 | 6,662 | 823,150 | 683,520 | 2,073 |
| Ordinary tools plus repoctx | 3 | 187.77 | 20 | 724,490 | 6,110 | 730,600 | 591,104 | 1,973 |

Cached input and reasoning output are reported separately as usage fields; they
are not added to the input-plus-output total. Exact wall times remain in the
archived records and scores.

The model alias was unresolved and provider-cache behavior was uncontrolled.
The pinned `repoctx` binary (`f0678223…e3d05b92`) reported revision
`1dc9657c30c06ff26320fead0a23d28fbd947781` and `modified: true`; its full
version record is archived in the protocol. This provenance does not make
setup, operator work, or an unmeasured build free. No original agent trial
invoked `repoctx compile` or `repoctx context`.

| Task | Ordinary tools | With repoctx | Frozen-key result |
| --- | --- | --- | --- |
| `f-doc-discovery-ignore` | 3/3 claims | 3/3 claims | pass in both conditions |
| `f-discover-then-read` | 4/4 claims | 4/4 claims | pass in both conditions |
| `f-context-go-relationships` | 5/7 claims | 5/7 claims | partial in both conditions |

Both context answers omitted the required `build` → `choose` →
`chooseSymbols` intermediate, and described bounded adjacent relationships
without stating that records include available source locations. The frozen key
therefore marks those two claims failed rather than inferring them from cited
files. The ordinary-tools visibility answer also says a focused Go test could
not run, but its seven recorded commands contain no `go test` invocation; that
unsupported statement is recorded separately and does not change its three
visibility claims.

The archived trial records show one observed tool command event with exit code
2: the ordinary-tools context trial attempted to read an unavailable external
Go-skill path, then completed the task normally. This counts nonzero command
events; it does not count failures masked by shell pipelines or control
operators. In the visibility treatment, two multiword literal `repoctx search`
queries returned zero results without a process failure; subsequent reads
supplied the answer. These are observed workflow details, not outcome scores.

## Separate operator walkthrough

The operator walkthrough is not an agent trial and is not pooled with the
table above. At a 3,000-byte discovery cap it returned a 2,751-byte incomplete
response with `overview_limit`, `result_limit`, and `output_limit`; a focused
read of `README.md:267:290` returned a complete 2,773-byte response. Compilation,
validation, context, and snapshot-pinned expansion succeeded. The walkthrough
checked all 11 returned source spans against source hashes and byte ranges; its
context result exposed `chooseSymbols` and its incoming `choose` neighbor. A
separate, post-replay operator EOF read then used the existing `:START:0`
syntax for `pkg/agentctx/units.go:223:0`; it returned 4,905 complete bytes for
lines 223–336 and passed the exact hash/byte-range check. That is the twelfth
checked span, not a revision of the original eleven. Commands, checks, scripts,
and stdout/stderr are archived, but the generated IR is intentionally excluded.

## Archive and next record

[`evals/runs/m6-07-20260922`](../../evals/runs/m6-07-20260922) contains the
protocol snapshots, runner snapshots, source-export hashes, sandbox probe,
raw trial events/stderr/finals/records, frozen-key scores, and the separate
operator walkthrough. It excludes authentication data, binaries, trial
workspaces, source copies, and generated IR.

A single fresh treatment replay with corrected guidance was recorded separately
in [`evals/runs/m6-07-relationship-replay-20260922`](../../evals/runs/m6-07-relationship-replay-20260922).
It used the same frozen context-task key, a guidance overlay, and only the
treatment condition; it is a targeted development replay, not paired, held-out,
or aggregate evidence. Its one trial took 77.053016 seconds and recorded 12
tool items, 535,308 input tokens, 3,165 output tokens, 465,664 cached input
tokens, and 666 reasoning-output tokens. Cached input and reasoning output are
separate usage fields, not additions to the input-plus-output total; setup,
operator work, and local build cost remain excluded. Its final answer again
scored 5/7: it did not explicitly state the `choose` to `chooseSymbols`
delegation or the available source sites in relationship records. It made no
`repoctx compile` or `repoctx context` invocation. Five nonzero raw command
events requested `read` endpoints beyond the observed file length:
`render.go:110→104`, `types.go:180→178`, `generation.go:180→174`,
`units.go:340→336`, and `graph.go:145→131`. This replay cannot alter the frozen
six-trial result or establish an improvement claim.

After the replay, the current CLI's oversized-range diagnostic, README, and
skill were updated to recommend the existing `PATH:START:0` through-EOF form;
a focused regression was added. Those changes were not measured in an agent
trial, so this record makes no agent-benefit claim for them.
