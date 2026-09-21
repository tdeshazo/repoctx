# M5-02 content-addressed incremental artifacts

M5-02 adds an optional caller-owned parse-fragment cache to the compiler API and
`repoctx compile -cache-dir`. The cache key binds the fragment format, complete
compilation profile, language, and full source hash. Fragment records contain
only file-local nodes, strings, symbols, edges, and diagnostics. Every compile
remaps those local strings and rebuilds paths, semantic IDs, links, manifests,
and forward/reverse graphs for the current snapshot.

The cache is opt-in and is not needed by the local CLI. It must be a private real
directory outside the indexed repository. Entries are size-bounded, decoded
strictly, checked against their key and payload hash, semantically validated,
and atomically replaced. These checks detect corruption; they do not authenticate
a cache controlled by an attacker.

## Correctness and invalidation

`TestIncrementalParseFragmentsMatchCleanCompilation` compares canonical JSON and
snapshot IDs after a cold build, an unchanged warm build, source modification,
addition, deletion, rename, `go.mod` change, and compilation-profile change.
Unchanged content is reusable across a rename, while the current path-derived
unit is rebuilt. A profile change produces misses. Additions and modifications
parse only new content; deletions disappear because no global generation state
is retained in fragments. Current links and both graph directions are always
rebuilt, covering reverse dependents without persisting snapshot-local dense IDs.

Negative tests reject a cache inside the indexed root and a group/world-writable
cache, and repair a corrupted entry as a miss. The storage test compiles the
declared fixture and enforces the 1.25× referenced-fragment limit.

## Measured result and limitation

The declared 1,000-file Go benchmark was run on Linux/amd64, Go
1.27.1-X:nodwarf5, with ten samples of three iterations:

```sh
go test ./pkg/compiler -run '^$' \
  -bench '^BenchmarkM5CompilerPath/files=1000/(complete_compilation|warm_incremental_compilation)$' \
  -benchmem -benchtime=3x -count=10
```

| Path | Median | Allocated bytes/op | Allocations/op | Cache/IR |
| --- | ---: | ---: | ---: | ---: |
| Clean compilation | 134.80 ms | 32.19 MB | 394,823 | — |
| Warm fragment cache | 275.10 ms | 41.94 MB | 248,295 | 1.068× |

Fragment reuse reduces allocation count by about 37% and satisfies the storage
cap, but per-file open/read/JSON decoding makes this tiny-file workload slower
and increases allocation volume. Therefore M5's complete-path performance gate
remains open. The roadmap records the resulting optimization target: evaluate a
bounded packed generation or in-memory aggregate after verified immutable-source
reuse, rather than presenting disk-fragment reuse as a serving win.

## Verification

The completion checks are:

```sh
go test ./...
go vet ./...
go build -o /tmp/repoctx-m5-02 .
/tmp/repoctx-m5-02 artifacts -root . \
  -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json
/tmp/repoctx-m5-02 artifacts -root . \
  -source docs/repoctx-artifacts.source.json \
  -o docs/repoctx-artifacts.json -check
```
