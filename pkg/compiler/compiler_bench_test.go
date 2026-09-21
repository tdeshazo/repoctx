package compiler

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tdeshazo/repoctx/internal/benchfixture"
	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

var (
	benchmarkRepository *ir.Repository
	benchmarkStrings    *ir.Strings
	benchmarkBytes      []byte
	benchmarkContents   map[string][]byte
)

func BenchmarkM5CompilerPath(b *testing.B) {
	for _, files := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("files=%d", files), func(b *testing.B) {
			root := b.TempDir()
			cache := b.TempDir()
			sourceBytes, err := benchfixture.Write(root, files)
			if err != nil {
				b.Fatal(err)
			}
			opts := Options{Root: root, MaxBytes: 2 << 20, MaxReadBytes: 256 << 20,
				MaxEntries: 100000, IgnoreDirs: defaultIgnoreDirs()}
			profile := compilationProfile(opts)
			sourceRoot, err := sourceroot.Open(root)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { sourceRoot.Close() })
			remaining := opts.MaxReadBytes
			manifest, contents, err := captureInputs(sourceRoot, profile, &remaining)
			if err != nil {
				b.Fatal(err)
			}
			parsed, strings := parseInputs(opts, manifest, contents)
			linkRepository(parsed, strings, "example.test/profile")
			artifact, err := json.Marshal(parsed)
			if err != nil {
				b.Fatal(err)
			}

			reportScale := func(b *testing.B) {
				b.ReportMetric(float64(files), "files")
				b.ReportMetric(float64(sourceBytes), "source-bytes")
				b.ReportMetric(float64(len(artifact)), "artifact-bytes")
			}
			b.Run("source_discovery", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					paths, err := inventory(sourceRoot, profile)
					if err != nil {
						b.Fatal(err)
					}
					if len(paths) != files {
						b.Fatalf("discovered %d files, want %d", len(paths), files)
					}
				}
			})
			b.Run("parsing", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkRepository, benchmarkStrings = parseInputs(opts, manifest, contents)
				}
			})
			b.Run("linking", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					repo, strings := parseInputs(opts, manifest, contents)
					b.StartTimer()
					linkRepository(repo, strings, "example.test/profile")
					benchmarkRepository, benchmarkStrings = repo, strings
				}
			})
			b.Run("validation", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					if err := parsed.Validate(); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("serialization", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkBytes, err = json.Marshal(parsed)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("source_verification", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkContents, err = LoadInputs(parsed, root, false, 2<<20, 256<<20)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("complete_compilation", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkRepository, err = Compile(opts)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("warm_incremental_compilation", func(b *testing.B) {
				cached := opts
				cached.CacheDir = cache
				_, coldStats, err := CompileWithStats(cached)
				if err != nil {
					b.Fatal(err)
				}
				if coldStats.Hits+coldStats.Misses != files {
					b.Fatalf("cache lookups = %d, want %d", coldStats.Hits+coldStats.Misses, files)
				}
				b.ReportAllocs()
				b.ResetTimer()
				reportScale(b)
				b.ReportMetric(float64(coldStats.ReferencedBytes), "cache-bytes")
				b.ReportMetric(float64(coldStats.ReferencedBytes)/float64(len(artifact)), "cache/IR")
				for i := 0; i < b.N; i++ {
					var stats CacheStats
					benchmarkRepository, stats, err = CompileWithStats(cached)
					if err != nil {
						b.Fatal(err)
					}
					if stats.Hits != files || stats.Misses != 0 {
						b.Fatalf("warm cache stats = %+v", stats)
					}
				}
			})
		})
	}
}
