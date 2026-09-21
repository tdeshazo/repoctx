# M4-07 checkpoint and evidence lifecycle

Date: 2026-09-21

## Outcome

The Python adapter now isolates persisted evidence in an exclusive owner-only
directory for each caller-supplied run ID. Exact bundles are fsynced to a unique
temporary file and atomically linked into the run, without replacing an existing
handle. Per-run byte, bundle-count, and retention limits bound storage.

The harness can create a bounded checkpoint containing its task, run and bundle
handles, snapshot identity, relevant symbol/unit IDs, unresolved questions,
next retrieval, trust, and expiry. No storage path is exposed. On resume, the
adapter verifies all declared file sizes and digests, then makes a
snapshot-pinned context request against the current source before registering
any saved handle. Stale validation leaves all checkpoint evidence unavailable.

Missing and expired references have distinct errors. Cleanup considers only
expired, registered bundles in the active run; it neither removes active files
nor traverses another run. The embedding harness remains responsible for
caller-scoped store permissions, globally unique run IDs, protecting checkpoint
state, retention policy, and eventual empty-directory removal. Both the README
and vendored skill direct generated evidence and checkpoints outside the indexed
repository.

## Verification

```sh
python3 -m unittest tests.test_tool_bridge_lifecycle \
  tests.test_tool_bridge_reference tests.test_m1_retrieval
PYTHONPATH=src python3 -m unittest discover -s tests
go test ./...
go vet ./...
repoctx artifacts -root . -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json -check
```

Focused tests cover checkpoint contents and bounds, successful and stale resume,
snapshot pinning, missing and expired evidence, active-reference preservation,
cross-run cleanup isolation, store/count bounds, run collisions, and interrupted
publication without a visible partial target.

Dogfooding the updated M4-08 catalog with an 8 KiB discovery budget returned no
result because the relevant roadmap window plus envelope did not fit. The
explicit omission signals were correct, but the available relevant evidence was
buried. This trace is assigned to the existing M4-09 diagnosis work rather than
duplicated as another roadmap item.
