# Parent documentation migration fixture

This fixture is a small, checked-in migration of the advisory documentation
bundle in `repository_context_program/` into the contracts consumed by
`repoctx`.

The mapping is deliberately authored without generated hashes, byte
coordinates, or generated navigation data:

- `schema: rcx.bundle/v1` becomes `version: repoctx.manifest/v1alpha3` and
  `namespace: rcx`.
- The parent discovery root becomes the `docs` source root.
- Selected canonical leaves become `document_sources` with
  `repoctx.frontmatter/v1alpha1` frontmatter. Their stable IDs retain the
  parent IDs (`rcx:target-state` and `rcx:foundations`).
- The parent bundle's generated maps and report provenance remain advisory
  source material; they are not silently promoted to executable authority.

`legacy-agent-context.yaml` is retained only as a rejection fixture. The
current loader must reject it instead of interpreting the legacy
`rcx.bundle/v1` schema.
