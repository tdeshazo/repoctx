package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

type repeated []string

func (v *repeated) String() string     { return strings.Join(*v, ",") }
func (v *repeated) Set(s string) error { *v = append(*v, s); return nil }

func contextCmd(args []string) {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	root := fs.String("root", "", "required path to the matching source repository")
	query := fs.String("query", "", "task text, symbol name, or path (lexical retrieval)")
	var symbols, allows, denies repeated
	fs.Var(&symbols, "symbol", "exact semantic ID; repeat for multiple required seeds")
	fs.Var(&allows, "allow", "permitted relative file/directory prefix; repeat; not a glob")
	fs.Var(&denies, "deny", "denied relative file/directory prefix; repeat; deny wins")
	snapshot := fs.String("expect-snapshot", "", "required index snapshot digest when expanding prior context")
	depth := fs.Int("depth", 1, "graph depth 0..4; never expand unit/unresolved hubs")
	direction := fs.String("direction", "both", "in, out, or both")
	rels := fs.String("relations", "calls,defines", "comma-separated traversal relation names")
	maxBytes := fs.Int("max-bytes", 32768, "hard limit on the complete uncompressed serialized payload")
	maxSymbols := fs.Int("max-symbols", 12, "maximum included symbol records, 1..64")
	maxCandidates := fs.Int("max-candidates", 128, "maximum graph candidates, 1..4096")
	maxRelations := fs.Int("max-relations", 48, "maximum relationship records, 1..256")
	maxSource := fs.Int64("max-source-bytes", 2<<20, "per-file verification read bound")
	maxRead := fs.Int64("max-read-bytes", 256<<20, "total verification read bound")
	format := fs.String("format", "json", "json for agents, markdown for readable review")
	out := fs.String("o", "-", "output file or stdout; no compression or extra JSON re-encoding")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		fatal(fmt.Errorf("usage: repoctx context -root DIR -query TEXT [options] REPO_IR"))
	}
	var kinds []ir.EdgeKind
	for _, r := range strings.Split(*rels, ",") {
		switch strings.TrimSpace(r) {
		case "calls":
			kinds = append(kinds, ir.EdgeCalls)
		case "defines":
			kinds = append(kinds, ir.EdgeDefines)
		case "imports":
			kinds = append(kinds, ir.EdgeImports)
		default:
			fatal(fmt.Errorf("unsupported traversal relation %q", r))
		}
	}
	repo, e := compiler.Read(fs.Arg(0))
	if e != nil {
		fatal(e)
	}
	result, e := agentctx.Build(repo, agentctx.Options{Root: *root, Query: *query, Symbols: symbols, ExpectedSnapshot: *snapshot, Depth: *depth, Direction: *direction, Relations: kinds, MaxSymbols: *maxSymbols, MaxCandidates: *maxCandidates, MaxRelations: *maxRelations, MaxBytes: *maxBytes, Format: *format, AllowPaths: allows, DenyPaths: denies, MaxSourceBytes: *maxSource, MaxReadBytes: *maxRead})
	if e != nil {
		fatal(e)
	}
	if e := writePayload(*out, result.Payload); e != nil {
		fatal(e)
	}
	fmt.Fprintf(os.Stderr, "context: %d symbols, %d evidence blocks, %d relationships; %d bytes; approximately %d tokens (byte/4 heuristic, NOT a token bound)\n", len(result.Bundle.Symbols), len(result.Bundle.Evidence), len(result.Bundle.Relationships), result.Usage.Bytes, result.Usage.Tokens)
}

func validateCmd(args []string) {
	if len(args) != 1 {
		fatal(fmt.Errorf("usage: repoctx validate REPO_IR"))
	}
	repo, e := compiler.Read(args[0])
	if e != nil {
		fatal(e)
	}
	snapshot, e := repo.SnapshotID()
	if e != nil {
		fatal(e)
	}
	fmt.Printf("valid %s\nsnapshot: %s\n", repo.Version, snapshot)
}

// Atomically replace files after the complete bundle passes budget checks. A
// failure before rename never replaces a previously valid output. Explicit
// output paths are controlled by the caller, not inferred from repository text.
func writePayload(path string, b []byte) error {
	if path == "" || path == "-" {
		_, e := os.Stdout.Write(b)
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".repoctx-context-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(temp, path)
}
