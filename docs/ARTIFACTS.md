# Optional artifact declarations

`pkg/artifacts.Decode(data, artifacts.Limits{})` accepts caller-supplied JSON
under `repoctx.artifacts/v1alpha1`. It returns a `*Catalog` or a nil catalog with
a bounded error. No CLI flag, reserved catalog filename, discovery, source
reading, source verification, command execution or context enrichment is added.
The [accepted design](design/m3-01-artifact-schema.md) defines the contract.

The model preserves declaration array order and duplicates, including conflicting
input claims and duplicate artifact IDs. `json.Marshal(catalog)` round-trips this
model; whitespace and JSON escape spelling are not preserved. Optional owner,
input hash and runner check ID use pointers to retain absence; required empty
arrays remain arrays. Newly constructed Go values must satisfy the wire contract
before a caller can decode their serialization; marshaling alone is not validation.

Every limit is inclusive. Zero selects its production ceiling, positive values
lower it, and negative or above-ceiling settings fail. Production ceilings are
1 MiB including whitespace, depth 16, 1024 artifacts, 4096 relationships, 64
applicability claims and inputs per artifact, 16 sources per object, and 16384
combined nested array entries. The byte guard runs before parsing or allocations
based on input content; syntax scanning checks depth and duplicate decoded keys
before object-shape decoding. Subsequent allocation and validation are bounded by
that byte ceiling. Errors identify trusted field/index paths, syntax container ordinals (`[index]` for arrays,
`{index}` for object members), or the JSON root and
never echo declaration values or unknown member names.

The [Draft 2020-12 schema](artifacts.schema.json) expresses the structural subset.
Use the decoder for strict JSON spelling, Unicode, duplicate keys, decoded UTF-8
byte lengths, namespace equality, span ordering, and aggregate/transport limits.
Schema validity and decoder acceptance both leave hashes, coordinates, ownership,
lifecycle, dependency claims and check IDs unverified. Source-byte verification
and authority/reference handling remain separate, deferred stages.

Run conformance and source-only compatibility checks with:

```sh
go test ./pkg/artifacts ./pkg/agentctx
python -m unittest discover -s tests -p test_artifacts_schema.py -v
```

The shared positive fixture and original source bytes are under
`pkg/artifacts/testdata/`. Go tests separately exercise strict decoder rules and
reachable/lower/isolated/masked resource boundaries. The Linux-specific inotify
test detects actual access to declared files and checks that an executable opaque
check ID is never run. Other platforms retain all portable structural tests.
