# M4-03 frozen experiment controls

Date: 2026-09-21

## Outcome

The M4 protocol now freezes the inputs needed before a run:

- a full clean Git revision and hashes of the corpus, repository fixtures,
  prompts, condition registry, scoring rules, protocol, and harnesses;
- the repoctx binary hash, reported build identity, v1alpha4 compiler contract,
  source provider profile, and v1alpha3 context profile;
- an explicit model ID, immutable model revision, reasoning profile, timeouts,
  byte/token/tool-call budgets, and per-track scoring rubrics;
- one permission profile shared by all matched end-to-end conditions and a
  separate no-tools/no-repository profile shared by every evidence supplier;
- a seeded globally shuffled schedule with unique fresh workspaces, explicit
  cold/warm preparation, and required before/after cache observations.

The artifact-aware condition is scheduled only for `ledger-lite`, the repository
with a declared catalog. The human oracle appears only in the evidence-only
track and remains labeled human-curated. Ordinary tools have no supplier cache;
their cache preparation is explicitly `not_applicable` rather than fabricated.

`scripts/m4_protocol.py lock` writes the generated lock outside the repository
and refuses dirty source, placeholder model identities, incompatible repoctx
contracts, and existing output. This keeps exact hashes, the schedule, and
binary metadata deterministic and harness-authored.

## Verification

```sh
python3 scripts/m4_protocol.py validate
python3 -m unittest tests.test_m4_protocol -v
python3 -m unittest tests.test_m4_conditions tests.test_m4_tasks -v
go test ./...
go vet ./...
```

This milestone fixes experimental controls but does not run the model study.
Complete cost accounting and aggregate decision thresholds remain M4-04 and
M4-05; no retrieval-quality or downstream-outcome claim is made.
