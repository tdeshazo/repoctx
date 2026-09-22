# M6-07 first-use tasks

This two-task corpus verifies the documented discover → inspect → optional-expand
workflow against `repoctx-source@20c335b3f0e6d82bdd6b0f52c605b1a7138e585e`.
It is workflow verification, not a comparative, efficiency, or generalization
claim.

Give each fresh agent the matching file in `agent_inputs/` and a fresh source
export at that revision. The export excludes evaluation, runner, historical
report, build, and repository-control material, while retaining source,
`README.md`, `ROADMAP.md`, and `skills/repoctx/` guidance. The EOF task checks
bounded-read recovery; the artifacts task directs indexed context followed by
source inspection to trace a concrete cross-file caller/callee chain.

`evaluator/expected.json` is operator-only and must not be given to an agent.
Before execution, verify all frozen inputs and the key from this directory:

```sh
sha256sum -c evaluator/freeze.sha256
```

Record source-linked response completeness and follow-up friction. Do not pool
these tasks with M4 or use them to make a statistical efficiency claim.
