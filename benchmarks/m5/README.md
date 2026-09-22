# M5 complete-path profile

This profile measures repoctx before M5 caching or serving changes. The
synthetic Go workload scales the same source shape across 10, 100, and 1,000
files so stage growth is comparable. Each file contains one type, two
functions, and one call. This is an engineering workload, not a claim about all
repositories or languages.

[`profile.json`](profile.json) freezes the workload, required stages, ten-sample
policy, fixed three-iteration benchtime, complete-path accounting, and the
performance target for later M5 work. Run the compiler and context benchmarks
serially to avoid shared-CPU interference:

```sh
go version
go test ./pkg/compiler -run '^$' -bench '^BenchmarkM5CompilerPath$' \
  -benchmem -benchtime=3x -count=10 > /tmp/m5-compiler.txt
go test ./pkg/agentctx -run '^$' -bench '^BenchmarkM5ContextPath$' \
  -benchmem -benchtime=3x -count=10 > /tmp/m5-context.txt
python3 scripts/m5_profile.py report \
  --input /tmp/m5-compiler.txt --input /tmp/m5-context.txt \
  --output /tmp/m5-profile.json
```

The report generator requires every declared size/stage pair to have exactly
ten samples, checks invariant artifact metrics, records raw-input and benchmark
harness hashes, and writes only outside the indexed repository.

If collecting a Go CPU or memory profile, pass both the profile path and test
binary path explicitly, for example `-cpuprofile=/tmp/m5-cpu.pprof -o
/tmp/m5-agentctx.test`. Go otherwise leaves a generated package test binary in
the working directory, where live discovery can mistake it for repository input.

## Measurement boundaries

- `source_discovery` inventories compilable paths without reading their bytes.
- `parsing` lowers captured bytes to unlinked IR; `linking` starts from a fresh
  parsed fixture, with fixture construction outside the benchmark timer.
- `validation` checks linked IR; `serialization` JSON-encodes it.
- `source_verification` performs strict local `LoadInputs` verification.
- `ranking` runs selection over already verified sources.
- `complete_compilation` includes discovery, reads, parsing, linking,
  validation, and the compiler's second verification pass, but no serialization.
- `warm_incremental_compilation` has the same boundary as complete compilation,
  but starts from a populated caller-owned parse cache. It still inventories,
  reads and verifies source, and rebuilds all global strings, links, and graphs.
  Its `cache-bytes` metric counts unique fragments referenced by that generation.
- `context_materialization` is the complete verified-local `Build`, including
  validation, source verification, ranking, bounded selection, rendering, and
  bundle validation. It is a composite measure, not an atomic stage.

The reported complete path is `complete_compilation + serialization +
context_materialization`. Its median and p95 component sum are descriptive sums
of independently sampled stages, not a latency distribution from paired
end-to-end trials. `B/op` is allocation volume, not peak resident memory.
Artifact growth compares compact canonical IR bytes with Go source-file bytes;
the small `go.mod` is excluded from the source-byte metric. Context
bytes remain bounded by the fixed query and 32 KiB output limit.

M5-03 adds a separate repeated-query benchmark so the frozen M5-01 stage set
does not silently change:

```sh
go test ./pkg/agentctx -run '^$' \
  -bench '^BenchmarkM5ImmutableGeneration$/^files=1000$' \
  -benchmem -benchtime=3x -count=10
```

`BenchmarkM5ImmutableGeneration` excludes its one-time strict verification and
generation construction from the timer. It measures context materialization
against privately owned, already verified index and source bytes. The ordinary
`context_materialization` benchmark remains the strict per-request local path.

M5-04 measures the bounded in-memory serving path, including compact substring
symbol discovery, selection of returned semantic IDs, and exact context
expansion through the active authorization-scoped generation:

```sh
go test ./pkg/agentctx -run '^$' \
  -bench '^BenchmarkM5WarmServing$/^files=1000$' \
  -benchmem -benchtime=3x -count=10
```

Generation construction and publication are outside the warm request timer.
The benchmark reports the deterministic retained-byte charge alongside canonical
IR bytes; this metric is a cache budget, not Go heap size or RSS.

## Predeclared M5 target

On the 1,000-file workload, subsequent warm-path work must reduce both the p95
complete-path component sum and median allocated bytes to at most 50% of this
cold baseline. Any incremental storage may add at most 1.25 times the canonical
IR bytes. Clean and reused paths must produce identical canonical semantic
output, and the warm path must retain verified source freshness. A timing or
allocation win that violates those correctness conditions does not meet the
target.
