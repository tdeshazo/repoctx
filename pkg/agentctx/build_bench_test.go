package agentctx

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/tdeshazo/repoctx/internal/benchfixture"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

var (
	benchmarkCandidates []candidate
	benchmarkResult     *Result
)

func BenchmarkM5ContextPath(b *testing.B) {
	for _, files := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("files=%d", files), func(b *testing.B) {
			root := b.TempDir()
			sourceBytes, err := benchfixture.Write(root, files)
			if err != nil {
				b.Fatal(err)
			}
			repo, err := compiler.Compile(compiler.Options{Root: root})
			if err != nil {
				b.Fatal(err)
			}
			irBytes, err := json.Marshal(repo)
			if err != nil {
				b.Fatal(err)
			}
			opts := Options{Root: root, Query: "Compute helper Record", Depth: 1,
				Direction: "both", Relations: []ir.EdgeKind{ir.EdgeCalls, ir.EdgeDefines},
				MaxBytes: 32768, MaxSymbols: 12, MaxCandidates: 128, MaxRelations: 48,
				MaxUnits: 8, MaxSourceBytes: 2 << 20, MaxReadBytes: 256 << 20,
				Consistency: "verified-local", Format: "json"}
			sources, err := loadSources(repo, opts)
			if err != nil {
				b.Fatal(err)
			}
			result, err := Build(repo, opts)
			if err != nil {
				b.Fatal(err)
			}

			reportScale := func(b *testing.B) {
				b.ReportMetric(float64(files), "files")
				b.ReportMetric(float64(sourceBytes), "source-bytes")
				b.ReportMetric(float64(len(irBytes)), "artifact-bytes")
				b.ReportMetric(float64(len(result.Payload)), "context-bytes")
			}
			b.Run("ranking", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkCandidates, _, _, err = choose(repo, opts, sources)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("context_materialization", func(b *testing.B) {
				b.ReportAllocs()
				reportScale(b)
				for i := 0; i < b.N; i++ {
					benchmarkResult, err = Build(repo, opts)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
