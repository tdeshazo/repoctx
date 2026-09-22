# Canonical repository manifest

`agent-context.yaml` is the repository-authored entry point for the canonical
repository model. The current contract is `repoctx.manifest/v1alpha1`, described
by [the JSON Schema](manifest.schema.json) and enforced by `pkg/manifest`.
Only this current active-development contract is supported.

The manifest declares an authored repository namespace plus:

- named source roots beneath the caller-selected repository root;
- semantic artifact-authoring inputs and their contract;
- components and their source, artifact, and provider inputs;
- supported derived views over those components; and
- the capabilities the repository expects the current compiler to provide.

The checked-in [repoctx manifest](../agent-context.yaml) is the example and
dogfood fixture. Validate it without compiling a repository:

```sh
repoctx manifest -root . -file agent-context.yaml
```

The command prints the validated semantic model as JSON. Declaration order is
preserved. The namespace and IDs are authored, stable names; they are not paths,
commands, account identities, or proof of ownership. IDs are unique across all
declaration kinds, and every reference must resolve to the expected kind.

## Authored versus generated data

The manifest contains only authored semantic declarations. Hashes, byte/line
coordinates, observed availability, Git state, compiler identity, provider
results, and snapshot IDs are generated data and are rejected as unknown
fields. `artifact_sources` points to the existing compact semantic authoring
format; it does not point to the generated catalog containing hashes and spans.

Structural validity grants no filesystem, provider, network, execution, or
policy authority. The caller selects `-root`; every path must remain beneath it.
Manifest validation opens declared paths only to establish availability and
type. It does not invoke providers or compile derived views.

## Strict YAML and limits

The decoder accepts one UTF-8 YAML document of at most 256 KiB. All mappings are
closed and all fields are required, including arrays that may be empty. Duplicate
keys, duplicate identities or references, aliases, anchors, explicit tags,
multiple documents, type coercion, unsafe paths, unknown references, unsupported
providers/formats/views/capabilities, and unavailable inputs fail the whole
manifest. Diagnostics are bounded and never quote repository values.

Production ceilings are 12 representation-graph levels, 4,096 total nodes, 64
source roots, 64 artifact sources, 256 components, 256 provider inputs, 64
derived views, and 64 capabilities. Callers may lower but not raise these limits.
Each component or view may contain at most 256 references.
File paths are relative slash paths without traversal, globs, backslashes,
colons, control characters, empty segments, or symlinks. Only a source-root path
may be `.`.

The schema validates the portable representation shape. Runtime checks add byte
length, YAML syntax, cross-reference, capability consistency, resource,
confinement, and availability rules that JSON Schema cannot express.
