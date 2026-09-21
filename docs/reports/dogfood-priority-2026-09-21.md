# Dogfood finding priority — 2026-09-21

## Ranking

1. **Executable provenance and capability preflight (M6-05, high).** The first
   `repoctx discover` attempt invoked an older binary on `PATH` and failed with
   `unknown command "discover"`. This blocks the workflow before evidence can
   be retrieved and can make checkout documentation appear incorrect.
2. **Bounded discovery result diversity (M4-10, medium).** A broad query spent
   its result/output budget on repeated high-overlap documentation windows and
   did not surface the implementation package. The response was explicitly
   incomplete and focused `files`/`read` follow-ups recovered the source, so the
   impact is degraded navigation rather than silent success.
3. **Readable excerpt boundaries (M4-11, low).** An indexed Go excerpt began in
   the middle of a token before the selected function. Exact coordinates,
   completeness labels, and the full selected definition remained available;
   readability suffered without corrupting evidence.

Priority considers workflow blockage first, then evidence recall, then
presentation quality. M4-10 and M4-11 remain evaluation items because changing
ranking or excerpt expansion without held-out measurements could reduce answer
quality or violate output budgets.

## M6-05 disposition

The shared CLI now provides `repoctx version` and `repoctx --version`.
`repoctx version -format json` emits `repoctx.build/v1alpha1` with release,
revision, modified state, distribution, Go/platform details, and supported wire
contracts. Go build information supplies VCS fields when available; unavailable
fields are explicitly `unknown`. Python package builds inject the project
release and `python-package` distribution while leaving revision unknown.

Both entry points share the command and have parity tests. The vendored agent
skill requires a preflight and explains that version output is descriptive, not
authenticated source identity. README packaging guidance makes the same trust
boundary explicit.

## Verification

- `go test ./...`: passed, including root/legacy help and version parity plus
  injected/unknown provenance cases.
- `python3 -m unittest tests.test_launcher -v` with `PYTHONPATH=src`: 3/3 passed.
- An isolated wheel build/install returned release `0.1.0`, distribution
  `python-package`, and explicit unknown revision/modified fields.
- A normal checkout binary returned the current Git revision and
  `modified:true`; `go run -buildvcs=false` returned explicit unknown fields.
- `python3 scripts/m0_baseline.py --output
  /tmp/repoctx-dogfood-priority-baseline.json`: 17/17 checks passed, including
  race, vet, both entry-point builds, and wheel/sdist version invocations.
- `gopls check` reported no diagnostics, `gopls vulncheck ./...` reported no
  known reachable vulnerabilities, and `git diff --check` passed.
- Dogfooding the packaged binary retrieved this report, the roadmap, README,
  and version tests while retaining explicit incomplete/omission signals.
