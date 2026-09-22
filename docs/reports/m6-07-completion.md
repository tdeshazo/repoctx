# M6-07 workflow verification

M6-07 is complete for the documented workflow checks after the 2026-09-22
actual-source treatment runs. The evidence verifies the documented
discover → inspect → expand path and bounded relationship follow-up; it does
not establish general agent success. The initial run used the source export and
pinned binary at
`20c335b3f0e6d82bdd6b0f52c605b1a7138e585e`, `gpt-5.6-luna` at medium
reasoning, a 300-second trial timeout, and an advisory 24-call cap. It was a
directed workflow check, not a paired comparison, efficiency measurement, or
natural-adoption study.

The two fresh tasks were frozen in the [completion task corpus](../../evals/first-use/completion/)
with the operator-only [scoring key](../../evals/first-use/completion/evaluator/expected.json):

- `f-read-eof-recovery`: the final answer passed all five frozen claims. It
  traced the bounded read, supplied the `pkg/agentctx/units.go:223:0` retry and
  inclusive `223–336` span, and named `incomplete`, `omissions`, and
  `warnings`.
- `f-artifacts-check-chain`: the final answer passed four of five claims. It
  correctly described the `-check` and write branches and the limits of
  indexed links, and it identified the relevant `runArtifacts` callsites, but
  its headline rendered the sibling calls `readBounded` and
  `artifacts.Generate` as an arrow chain. The headline remains a source
  relationship error.

| Observation | Model / effort | Result | Calls | Wall seconds | Input + output tokens |
| --- | --- | --- | ---: | ---: | ---: |
| Fresh EOF recovery | `gpt-5.6-luna` / medium | 5/5 frozen claims | 7 | 57.334480 | 252,619 |
| Fresh artifacts chain | `gpt-5.6-luna` / medium | 4/5 frozen claims | 10 | 77.505481 | 341,046 |
| Same-task skill replay | `gpt-5.6-luna` / medium | 4/5 frozen claims | 12 | 108.330363 | 438,897 |
| Final tuned same-task check | `gpt-6-sol` / high | 5/5 frozen claims | 16 | 98.113844 | 387,484 |

All completed trials exited 0, stayed within the advisory cap, and left the
source unchanged. The initial artifacts task had one failed indexed context request
caused by an initially guessed symbol ID before a query-based context request
succeeded. The EOF task intentionally observed the oversized-range failure
before the successful EOF retry. Cached-input and reasoning-output fields are
retained in the raw records.

A same-task artifacts replay with revised skill guidance was run separately
under [`m6-07-chain-replay-20260922`](../../evals/runs/m6-07-chain-replay-20260922).
It is not independent evidence. The replay corrected the detailed edge bullets
but retained the same incorrect sibling arrow in its headline, so it did not
close the M6-07 relationship-trace gate. A final tuned replay of the same
frozen task and key, with the model changed to `gpt-6-sol` at high effort and
the revised skill guidance applied, then passed all five claims. It is
archived under [`m6-07-chain-verification-20260922`](../../evals/runs/m6-07-chain-verification-20260922).
This replay is not independent evidence and cannot attribute the pass to the
skill guidance alone. One trial command recorded `git rev-parse HEAD` exit 128
because the source export had no `.git` directory. The original archive is
[`m6-07-completion-20260922`](../../evals/runs/m6-07-completion-20260922).

The earlier [first-use report](m6-07-first-use.md) remains the regression
reference for prior cross-file omissions and read-range friction. The final
bounded check required explicit caller/callee edge pairs or an equivalent
representation that cannot turn sibling calls into a chain. The final tuned
replay supports only the documented workflow checks described here. These directed tasks do not
establish agent efficiency, broad task success, or natural workflow adoption.
