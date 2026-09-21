# M5-01 complete-path profile

M5-01 adds repeatable Go benchmarks and a deterministic report generator for a
10/100/1,000-file synthetic Go workload. The frozen contract and future target
are in [`benchmarks/m5/profile.json`](../../benchmarks/m5/profile.json); the
measurement boundaries and reproduction commands are documented in the
[`benchmarks/m5` README](../../benchmarks/m5/README.md).

The baseline was collected on Linux/amd64 with Go 1.27.1-X:nodwarf5 on an AMD
Ryzen 5 PRO 4650U. Each size/stage has ten samples of three iterations. Times
below are medians; memory is allocated bytes per operation, not peak RSS.

## Scale profile

| Files | Source | Canonical IR | IR/source | Complete path | Allocated | Context |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10 | 1,570 B | 28,937 B | 18.43x | 9.35 ms | 1.26 MB | 14,480 B |
| 100 | 15,700 B | 277,379 B | 17.67x | 59.47 ms | 7.64 MB | 16,319 B |
| 1,000 | 157,000 B | 2,787,536 B | 17.76x | 352.24 ms | 78.56 MB | 16,279 B |

Canonical IR size grows approximately linearly on this deliberately small,
repetitive source shape. Its high source ratio reflects structural metadata on
tiny files, not compression behavior on typical repositories. The selected
context remains near 16 KiB because query and output bounds are fixed.

## 1,000-file attribution

| Stage | Median | Allocated |
| --- | ---: | ---: |
| Source discovery | 16.54 ms | 0.45 MB |
| Parsing | 51.98 ms | 22.84 MB |
| Linking | 18.73 ms | 3.27 MB |
| Validation | 4.37 ms | 0.71 MB |
| Serialization | 31.63 ms | 8.39 MB |
| Source verification | 57.72 ms | 6.24 MB |
| Ranking | 27.04 ms | 9.65 MB |

Source verification is the largest atomic latency stage. Parsing is the
largest atomic allocation stage. Complete compilation measures 133.67 ms and
32.31 MB; complete verified context materialization is a composite 186.94 ms
and 37.86 MB. These findings prioritize verified source reuse and incremental
parsing without assuming either may weaken source identity or canonical output.

## Target and limits

For the 1,000-file workload, later M5 work must bring the warm p95 complete-path
component sum and median allocation volume to at most 50% of this cold baseline,
while incremental storage stays within 1.25 times canonical IR bytes. Clean and
reused builds must remain canonically identical and sources must remain verified.
The measured p95 component sum is 399.94 ms, so the warm ceiling is 199.97 ms;
the allocation ceiling is 39.28 MB and incremental-artifact ceiling is
3,484,420 bytes.

The complete-path number sums independently sampled compiler, serialization,
and context-materialization medians; it is not a paired end-to-end latency
distribution. The workload covers Go only and does not establish production or
cross-language performance. No optimization or service is introduced by M5-01.
