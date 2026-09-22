# M6-03 distribution validation

M6-03 is complete for the only advertised distribution target: Linux amd64.
No other operating system, architecture, or portable wheel is claimed.

The existing M0 baseline runner is the canonical distribution harness; extending
it avoids a second packaging test path. It now builds both Go entry points and
compares their help and version contracts. It also builds and installs the Python
wheel and source distribution into separate temporary targets. Each installed
target is exercised from outside the checkout with only that target on
`PYTHONPATH` and an empty `PATH`, proving that runtime use does not require the Go
toolchain or C compiler used to build the native binary. Both `python -m
repoctx_cli` and the console script must agree on build information, and the
installed command must complete help and repository-overview operations.

The validated source state passed on Linux amd64 with Go 1.27.1, GCC 16.2.1,
CPython 3.14.7, `build` 1.6.1, and pip 26.2.1. The wheel and installed native
binary remain platform-specific. Source and sdist builds require Go 1.23 or
newer, CGO, and a working C compiler; an installed wheel does not.

Reproduce from a clean checkout with the declared prerequisites:

```sh
python3 scripts/m0_baseline.py --output /tmp/repoctx-m6-03.json
```

The run completed 39 checks with no failures, including tests, race detection,
vet, schemas, fixtures, both Go entry points, both Python package formats, both
Python launch forms, isolated imports, current build metadata, and runtime smoke
operations. The JSON output is a generated run artifact and is intentionally not
duplicated in this report.
