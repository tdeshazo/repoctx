# M6-06 stale roadmap automation retirement

Date: 2026-09-21.

M3-03 dogfooding found that `next-roadmap-item.yaml` and
`docs/workflows/next-roadmap-item.md` remained pinned to completed M3-01. The
maintainer confirmed that these Comanda workflows are no longer used. Updating
or adding validation would preserve an unsupported execution path, so M6-06 is
resolved by removing both the executable workflow and its operator guide.

A repository reference audit found no active caller or documentation link that
depends on either file. Remaining textual mentions are historical evidence in
retained reports and baseline snapshots; those records are intentionally not
rewritten.

Verification:

- `rg -n "next-roadmap-item|docs/workflows/next-roadmap-item"` confirms only
  historical evidence and this retirement record remain;
- `go test ./...`;
- `PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'` (27
  tests passed); and
- `git diff --check`.
