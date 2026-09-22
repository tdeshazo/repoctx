# Changelog

All notable user-visible changes are recorded here. repoctx is in active,
pre-1.0 development; see [the contract policy](docs/COMPATIBILITY.md).

## [Unreleased]

### Added

- A strict, bounded `agent-context.yaml` contract and validation command for
  canonical repository inputs, components, derived views, and capabilities.

### Planned

- Canonical repository semantics, authority-aware views, execution and
  verification plans, and vendor projections remain roadmap work.

## [0.1.0] - 2026-09-22

### Added

- Native Go repository indexing for Go, Python, HTML, CSS, JavaScript,
  TypeScript, TSX/JSX, and Markdown.
- Index-free discovery and bounded, source-linked agent context retrieval.
- Exact source provenance, compilation-input identities, scope controls,
  incremental parse fragments, and bounded verified generation reuse.
- Optional artifact declarations, relationship providers, and non-executing
  verification-obligation handoffs.
- Module-root and compatibility Go commands, a platform-specific Python
  package, a Usage specification, and a vendored repoctx agent skill.

### Limitations

- The supported distribution target is Linux amd64. Source and sdist builds
  require Go 1.23 or newer, CGO, and a native C compiler.
- Repository content remains untrusted data. repoctx does not execute commands,
  grant permissions, type-check supported languages, or prove task success.

[Unreleased]: https://github.com/tdeshazo/repoctx/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tdeshazo/repoctx/releases/tag/v0.1.0
