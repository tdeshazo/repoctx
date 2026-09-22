---
version: repoctx.frontmatter/v1alpha1
entities:
  - id: maintainers
    kind: owner
    owners: []
    scopes: []
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - agent-context.yaml
  - id: manifest-document
    kind: document
    owners: [maintainers]
    scopes:
      - kind: file
        path: docs/MANIFEST.md
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - docs/MANIFEST.md
  - id: manifest-explicit-opt-in
    kind: decision
    owners: [maintainers]
    scopes:
      - kind: subtree
        path: .
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - agent-context.yaml
  - id: canonical-manifest
    kind: contract
    owners: [maintainers]
    scopes:
      - kind: file
        path: agent-context.yaml
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - docs/manifest.schema.json
  - id: exact-declaration-source
    kind: requirement
    owners: [maintainers]
    scopes:
      - kind: subtree
        path: pkg/manifest
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - pkg/manifest/entities.go
  - id: manifest-conformance
    kind: verification_obligation
    owners: [maintainers]
    scopes:
      - kind: subtree
        path: pkg/manifest
    lifecycle: active
    supersedes: []
    sensitivity: public
    freshness:
      inputs:
        - pkg/manifest/manifest_test.go
---

# Canonical repository manifest

`agent-context.yaml` is the repository-authored entry point for the canonical
repository model. The current contract is `repoctx.manifest/v1alpha3`, described
by [the JSON Schema](manifest.schema.json) and enforced by `pkg/manifest`.
Typed Markdown declarations use `repoctx.frontmatter/v1alpha1` and its
[frontmatter schema](frontmatter.schema.json).
Only this current active-development contract is supported.

The manifest declares an authored repository namespace plus:

- named source roots beneath the caller-selected repository root;
- named Markdown inputs containing typed semantic frontmatter;
- semantic artifact-authoring inputs and their contract;
- components and their source, artifact, and provider inputs;
- declaration-only component dependencies via `depends_on`;
- supported derived views over those components; and
- the capabilities the repository expects the current compiler to provide.

Canonical compilation remains explicit:

```sh
repoctx compile -root . -manifest agent-context.yaml -o repo.ir.json.gz
```

Without `-manifest`, compilation emits the existing bounded source-only IR.

The checked-in [repoctx manifest](../agent-context.yaml) is the example and
dogfood fixture. Validate it without compiling a repository:

```sh
repoctx manifest -root . -file agent-context.yaml
```

The command prints the validated semantic model as JSON. Declaration order is
preserved. The namespace and IDs are authored, stable names; they are not paths,
commands, account identities, or proof of ownership. IDs are unique across all
declaration kinds, and every reference must resolve to the expected kind.

`document_sources` explicitly lists the Markdown files whose leading YAML
frontmatter is semantic input. Frontmatter may declare documents, components,
decisions, contracts, requirements, verification obligations, and owners. Each
entity also declares owner IDs, file/subtree scopes, lifecycle, supersession,
sensitivity, and freshness inputs. Components carry the same semantic fields
directly in the manifest. Compilation qualifies local IDs with the namespace,
sorts by ID, resolves owner and same-kind supersession references, and attaches
an exact hashed byte span with status `declared`.

`artifact_sources` are compiled through the same canonical entity path. Their
compact declarations can carry multiple exact source anchors and declared
relationships. Duplicate entity IDs across manifest components, artifact
authoring, or Markdown frontmatter reject the whole compilation; source order
never selects a winner.

Component `depends_on` entries are namespace-qualified declaration-only
relationships. They must name another component in this manifest and cannot
name the component itself; they do not establish execution order, runtime
dependency resolution, or transitive closure.

## Authored versus generated data

The manifest contains only authored semantic declarations. Hashes, byte/line
coordinates, observed availability, Git state, compiler identity, provider
results, and snapshot IDs are generated data and are rejected as unknown
fields. `artifact_sources` points to the existing compact semantic authoring
format; it does not point to the generated catalog containing hashes and spans.
The standalone generated catalog is a deterministic compatibility view of that
same input, not a second semantic declaration source.

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
source roots, 64 document sources, 64 artifact sources, 256 components, 256 provider inputs, 64
derived views, and 64 capabilities. Callers may lower but not raise these limits.
Each component or view may contain at most 256 references.
File paths are relative slash paths without traversal, globs, backslashes,
colons, control characters, empty segments, or symlinks. Only a source-root path
may be `.`.

The schema validates the portable representation shape. Runtime checks add byte
length, YAML syntax, cross-reference, capability consistency, resource,
confinement, and availability rules that JSON Schema cannot express.
