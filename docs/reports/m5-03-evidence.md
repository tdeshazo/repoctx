# M5-03 verified immutable source reuse

M5-03 adds an application-owned `VerifiedSourceGeneration` for repeated Go API
queries. `VerifySourceGeneration` performs strict `verified-local` inventory,
auxiliary-input, and two-pass source verification once. It then owns a bounded
defensive copy of the index, exact source bytes, line maps, and canonical
snapshot/source/profile identities. `BuildFromGeneration` reuses those bytes
without filesystem reads.

Reuse is explicit rather than ambient. Every request must provide the caller-
authenticated generation `SnapshotID`; a mismatch fails before selection.
Explicit allow/deny scope must match the captured compilation profile. Private
generation fields prevent ordinary caller mutation, and tests verify that later
filesystem changes and mutation of the original index cannot alter returned
evidence. Authentication, authorization, retention, and disposal remain outside
repoctx. Ordinary `Build` and the one-shot CLI retain strict local verification.

## Correctness coverage

The focused tests establish that:

- generation construction rejects nil, stale, incomplete, oversized, or
  unrooted inputs;
- selected symbols, retrieval units, relationships, exact evidence, source maps,
  and content identities match a strict local build;
- missing or mismatched authenticated snapshot pins and incompatible scopes fail;
- a generation owns its index and source bytes across later caller/filesystem
  mutations; and
- the existing `immutable` filesystem mode continues to hash source bytes, while
  only `BuildFromGeneration` skips later filesystem access.

## Measurement

The strict and generation-backed 1,000-file context paths were each measured
serially on Linux/amd64, Go 1.27.1-X:nodwarf5, AMD Ryzen 5 PRO 4650U, with ten
samples of three iterations:

```sh
go test ./pkg/agentctx -run '^$' \
  -bench '^BenchmarkM5ContextPath$/^files=1000$/^context_materialization$' \
  -benchmem -benchtime=3x -count=10
go test ./pkg/agentctx -run '^$' \
  -bench '^BenchmarkM5ImmutableGeneration$/^files=1000$' \
  -benchmem -benchtime=3x -count=10
```

| Context path | Median | Allocated bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| Strict verified-local | 197.97 ms | 37.98 MB | 172,348 |
| Verified generation | 92.21 ms | 25.50 MB | 93,776 |

Generation reuse reduces this measured median by 53.4%, allocated bytes by
32.9%, and allocation count by 45.6%. Construction and its strict verification
are deliberately outside the repeated-query timer and must be amortized by the
application. The overall M5 complete-path gate remains open: compilation,
serialization, and generation-backed materialization together have not yet been
shown to meet the predeclared 50% p95 and allocation ceilings. M5-04 owns bounded
retention/eviction and any packed or aggregate warm compilation design.
