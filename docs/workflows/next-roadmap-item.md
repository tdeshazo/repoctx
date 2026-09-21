# Complete M3-01 with Comanda

[next-roadmap-item.yaml](../../next-roadmap-item.yaml) targets **M3-01 — Define a
minimal optional artifact schema**, the first unchecked work item in
[ROADMAP.md](../../ROADMAP.md). It remains pinned to M3-01 across retries and
does not complete the rest of M3.

Run from the repository root with Comanda's `openai-codex` provider configured:

```sh
comanda validate next-roadmap-item.yaml
comanda chart next-roadmap-item.yaml
comanda process next-roadmap-item.yaml --live
```

Validation runs preflight checks without executing agents or tests. Processing
edits code, tests and documentation and creates local commits. It requires a
working coding-agent provider, Git identity, the Go/C/Python toolchains and
[Python test dependencies](../../requirements-test.txt) used by the
[baseline runner](../M0_BASELINE.md). Start with a clean checkout where possible;
agents are instructed to preserve existing changes. No publication is requested.

Each iteration plans the contract, independently reviews its design, implements
accepted work, reviews and corrects findings, runs the baseline as a deterministic
tool step, independently audits the result, and records accepted progress.
Rejected designs return through review in the next iteration before implementation.
The baseline covers Go tests, race tests, vet, both entry-point builds, existing
schemas, Python distribution/launcher checks and deterministic fixtures. Dedicated
tests for the new artifact contract are also required by the implementation and
review steps; the historical baseline alone cannot establish M3-01 acceptance.

Execution creates a design record at `docs/design/m3-01-artifact-schema.md`, a
durable handoff at `docs/reports/m3-01-evidence.md`, and a baseline report at
`artifacts/m3-01-baseline.json`. Accepted bookkeeping retains the report at
`docs/reports/m3-01-baseline.json`. Inspect and retain failed-run artifacts too.
Only independently accepted M3-01 work may change its roadmap checkbox, with
links to implementation revisions, design review and reproducible checks.

The workflow uses Comanda's
[stateful agentic loop](https://github.com/kris-hansen/comanda#a-loop-does-not-finish-because-an-agent-says-done)
with checkpoints each iteration and a 12-iteration limit. There is no overall
loop deadline; the baseline tool step has a one-hour timeout. Inspect state with:

```sh
comanda loop status repoctx-roadmap-m3-01-artifact-schema
```

After interruption or a failed deterministic gate, inspect the report, address
the failure and rerun the same process command. A failed tool step stops before
completion bookkeeping; fixing it may require a separate repair before resuming.
Resume is not a transaction or rollback: every iteration must reconcile actual
files and commits with durable evidence. Avoid concurrent runs of this named
loop, including in other checkouts sharing Comanda's user state.

The final marker distinguishes `CONTINUE`, `ROADMAP_ITEM_BLOCKED`, and
`ROADMAP_ITEM_COMPLETE`. Both terminal markers stop the loop; a stopped process
or iteration limit does not itself mean the roadmap item is complete. Read the
final verdict and linked evidence. Missing dependencies or rejected design work
must never be converted into passing results.

Repository-wide paths allow the workflow to choose the smallest appropriate
implementation after design review. Its scope and preservation instructions are
agent constraints, not a filesystem sandbox or an authority conferred by the
artifact schema being designed.
