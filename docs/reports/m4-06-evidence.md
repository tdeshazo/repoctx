# M4-06 optional file-reference prototype

Date: 2026-09-21

## Outcome

The model-neutral Python adapter now keeps inline delivery as its default and
offers opt-in persisted delivery through a caller-created directory outside the
indexed repository. A caller-supplied opaque handle names exact context stdout
bytes published without replacement at mode `0600`; the model-facing reference
does not disclose the storage path.

The bounded reference includes snapshot identity, byte count and SHA-256,
relevant semantic symbol/unit IDs, trust classification, source-bundle
omissions, reference-level ID omissions, and read limits. It explicitly has no
preview, so retrieval guidance cannot be mistaken for evidence. If relevant IDs
exceed the reference budget, IDs are removed and counted rather than silently
overflowing the response.

`read_context` accepts only a handle registered by the same adapter instance.
It rechecks the persisted regular file's size and digest, then returns a bounded
base64 range with offsets and chunk identity. Base64 makes arbitrary byte
boundaries exact, including boundaries inside UTF-8. The implementation budgets
both source bytes and the complete serialized read response, and rejects a
budget too small to make forward progress.

The prototype prevents replacement of an existing handle, preserves a
pre-existing colliding file, and rejects path-like handles and stores inside the
indexed source root. Store provisioning, run-scoped namespaces, retention,
cleanup, checkpoints, and resume validation remain the explicit scope of M4-07.

## Verification

```sh
python3 -m unittest tests.test_tool_bridge_reference tests.test_m1_retrieval
python3 -m unittest discover -s tests
go test ./...
go vet ./...
```

Focused tests reconstruct the original payload from multiple bounded reads,
exercise reference trimming and both omission sources, and cover collisions,
mutation, handle validation, store isolation, inline compatibility, and unit-ID
forwarding.

Dogfooding also found that discovery split the exact `M4-07` identifier into
common tokens and spent its result budget on incidental generated-JSON hash
matches. The reproducible buried/over-retrieved trace is assigned to the
existing M4-09 diagnosis work rather than duplicated as another roadmap item.
