# Release maintenance

The release process validates and packages the single advertised target, Linux
amd64. It does not publish a Git tag, GitHub release, Python package, signature,
or security advisory. Those remain deliberate maintainer actions.

## Prerequisites

Use a clean checkout with Go 1.23 or newer, CGO and a native C compiler, Python
3.9 or newer, the packages in `requirements-test.txt`, and `govulncheck`. The
release version must match `pyproject.toml` and have a dated entry in
`CHANGELOG.md`.

## Verify

Run the complete baseline and the release-specific synchronization checks:

```sh
python3 scripts/m0_baseline.py --output /tmp/repoctx-release-baseline.json
python3 scripts/release.py check
```

The release check verifies Go module checksums and vulnerabilities, CLI/Usage
command parity, current schema identifiers, dependency notices, changelog and
packaging metadata, skill preflight commands, and required maintenance links.
Failures block release preparation rather than silently omitting a check.

## Build

Choose a new directory outside the repository:

```sh
python3 scripts/release.py build --version 0.1.0 \
  --output-dir /tmp/repoctx-release-0.1.0
```

The command reruns release checks, rejects a dirty checkout or in-repository
output, passes the source commit epoch to package builders, normalizes archive
metadata, and emits:

- a Linux amd64 native archive containing the executable and notices;
- the platform-specific Python wheel and source distribution;
- `SHA256SUMS`; and
- an unsigned provenance statement binding artifacts to the Git revision,
  source materials, toolchains, platform, and current repoctx contracts.

Checksums detect changed bytes but do not authenticate the publisher. The
provenance statement explicitly records that it is unsigned. Sign and publish
artifacts only through a maintainer-approved release channel, then create the
matching `vVERSION` tag and verify uploaded bytes against `SHA256SUMS`.

The sdist contains current source, schemas, maintenance documents, skill, tests,
and concise current release evidence. Historical raw evaluation transcripts
remain excluded; they are not required to build or inspect the release.

## Maintain

Before the next release, update `CHANGELOG.md`, dependency notices, supported
platform claims, schemas and contract identifiers, the Usage specification,
the canonical manifest, the vendored skill, and applicable validation reports.
Re-run both commands above from the final clean revision; do not reuse checksums
or provenance from an earlier commit.
