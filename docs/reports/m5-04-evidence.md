# M5-04 bounded warm serving

M5-04 adds a process-local Go library cache rather than a daemon. A
`GenerationStore` retains immutable `VerifiedSourceGeneration` values under
mandatory generation-count and deterministic byte bounds. It uses LRU eviction,
starts empty after process restart, creates no goroutines, and atomically makes
only a fully verified generation active. An evicted generation remains valid for
a request that already acquired it.

Every publish, lookup, discovery, and build carries a caller-owned authorization
scope. The store partitions keys by that scope, returns the same generic
`ErrGenerationUnavailable` for absent or inaccessible generations, and binds the
scope into context task identity. Authentication and authorization remain the
embedding application's responsibility; repository content cannot choose a
scope.

## Compact discovery and evidence expansion

Each generation builds two compact integer orderings over its immutable symbol
table: semantic ID and name. Exact discovery uses binary search. Substring and
Go-regex discovery scan semantic-ID order. Every mode inspects at most the
required `MaxScanned` bound, applies optional language, kind, path-prefix, and
unit filters, and enforces a hard result limit. Results expose snapshot ID,
semantic ID, name, language, kind, path, unit, and declaration span, but no
source body.

Limit and unscanned counts remain explicit and set `incomplete`. Returned IDs
feed `GenerationStore.Build`, which uses the existing context evidence path;
there is no parallel declaration or source representation. Explicit symbol-only
expansion avoids deriving unrelated document units, which profiling identified
as unnecessary work for this flow.

Tests cover exact/name, substring, and regex matching; filters; deterministic
ordering; result and scan omissions; source-free JSON; and expansion of a
discovered ID into exact context evidence. Invalid regexes, paths, bounds, nil
contexts, and cancelled requests fail.

## Isolation and lifecycle checks

Focused tests establish:

- cross-scope lookups cannot observe a generation, and identical snapshots in
  two scopes receive distinct task identities;
- cancelled publication leaves the previous generation active, while successful
  publication swaps the complete generation;
- count/byte eviction removes the old active lookup without invalidating an
  in-flight immutable pointer;
- a new store after restart exposes no prior generation;
- cancellation is observed before lookup/publication and during context
  materialization; and
- concurrent readers, discovery, and publication pass the Go race detector.

## Measured result

The declared 1,000-file synthetic Go workload was measured on Linux/amd64 with
Go 1.27.1-X:nodwarf5 on an AMD Ryzen 5 PRO 4650U. Ten samples of three
iterations include bounded substring discovery, selection of its three semantic
IDs, and context expansion through the active store. One-time compilation,
strict generation verification, and publication are outside the warm timer.

```sh
go test ./pkg/agentctx -run '^$' \
  -bench '^BenchmarkM5WarmServing$/^files=1000$' \
  -benchmem -benchtime=3x -count=10
```

| Path | Median | p95 | Median allocated bytes/op | Retained generation / canonical IR |
| --- | ---: | ---: | ---: | ---: |
| M5-01 cold complete-path baseline | 352.24 ms | 399.94 ms | 78.56 MB | — |
| M5-04 warm discover + expand | 56.04 ms | 61.38 ms | 20.67 MB | 1.224x |

The warm p95 is 15.3% of the cold p95 ceiling basis, and median allocation is
26.3% of the cold baseline; both are below the predeclared 50% limits. The
3,412,112-byte retained generation includes the 2,787,536-byte canonical IR,
verified source/line maps, and compact symbol orderings. The independent M5-02
parse-fragment cache remains 1.068x canonical IR. Both retained-generation and
incremental-artifact measures are below the 1.25x cap. Retained bytes are a
deterministic cache charge, not heap size or RSS.

The cold numbers remain the previously published M5-01 samples rather than a
new favorable baseline. This workload is repetitive Go source and does not
claim universal production latency. The ordinary CLI remains one-shot and does
not depend on the library store, a service, or an external database.
