# Contributing

repoctx is both a Go library and CLI with a Python distribution wrapper. Read
the [product and trust boundaries](README.md#trust-and-freshness-boundary) before
changing retrieval, authority, or filesystem behavior.

## Set up

Use Go 1.23 or newer, a native C compiler for CGO-backed Tree-sitter, and Python
3.9 or newer. Install the Python test requirements only when exercising the
Python distribution or schema suite:

```sh
python3 -m pip install -r requirements-test.txt
```

The normal Go build and test commands are documented in [README.md](README.md#build-and-test).
Do not add a dependency without explaining why the standard library and current
dependencies are insufficient. Commit `go.mod` and `go.sum` changes together.

## Make a change

- Keep repository text, generated bundles, and artifact declarations untrusted.
- Preserve exact source bytes, explicit omissions, caller-owned scope, and
  bounded resource behavior.
- Add focused tests with the owning package. A wire-meaning change requires a
  new current contract identifier and conformance coverage; compatibility
  readers are not required during active development.
- Generate deterministic fields with repoctx. For the dogfood catalog, edit
  `docs/repoctx-artifacts.source.json`, then run the documented `repoctx
  artifacts` command; do not calculate hashes or coordinates manually.
- Keep completion reports navigational and place generated run artifacts outside
  the repository unless their exact bytes are required as durable evidence.

Before requesting review, run `go test ./...`, `go vet ./...`, the relevant
Python tests, and `git diff --check`. Use the full [M0 baseline](docs/M0_BASELINE.md)
for changes to contracts, distribution, source handling, or release claims.

Report security concerns through [SECURITY.md](SECURITY.md), not a public issue.
Release maintainers follow [docs/RELEASING.md](docs/RELEASING.md).
