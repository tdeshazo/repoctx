package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/tdeshazo/repoctx/pkg/discovery"
)

func discoveryCmd(operation string, args []string) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runDiscovery(ctx, operation, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		var usage *discovery.UsageError
		if errors.As(err, &usage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func runDiscovery(ctx context.Context, operation string, args []string) error {
	o := discovery.DefaultOptions()
	o.Operation = operation
	fs := flag.NewFlagSet(operation, flag.ContinueOnError)
	fs.StringVar(&o.Root, "root", o.Root, "selected root; never expands to a parent Git root")
	fs.StringVar(&o.Format, "format", o.Format, "json or markdown (same discovery contract)")
	fs.BoolVar(&o.Hidden, "hidden", false, "include hidden entries, except .git")
	fs.BoolVar(&o.NoIgnore, "no-ignore", false, "disable local ignore rules; never bypass scope")
	var allows, denies repeated
	fs.Var(&allows, "allow", "permitted relative file/directory prefix; repeat; not a glob")
	fs.Var(&denies, "deny", "denied relative file/directory prefix; repeat; deny wins")
	fs.IntVar(&o.MaxBytes, "max-bytes", o.MaxBytes, "maximum final serialized payload bytes")
	fs.Int64Var(&o.MaxSourceBytes, "max-source-bytes", o.MaxSourceBytes, "maximum bytes per file")
	fs.Int64Var(&o.MaxReadBytes, "max-read-bytes", o.MaxReadBytes, "maximum total source and ignore-file bytes")
	fs.IntVar(&o.MaxResults, "max-results", o.MaxResults, "maximum result records")
	fs.IntVar(&o.MaxEntries, "max-entries", o.MaxEntries, "maximum directory entries enumerated (up to 10000000)")
	out := fs.String("o", "-", "output file or - for stdout; no index or cache is created")
	if operation == "overview" || operation == "discover" {
		fs.IntVar(&o.Depth, "depth", o.Depth, "overview depth; 0 omits the tree (inventory still scanned)")
	}
	if operation == "search" || operation == "discover" {
		fs.StringVar(&o.Query, "query", "", "search pattern or discover task text")
		fs.IntVar(&o.ContextLines, "context-lines", o.ContextLines, "surrounding lines per matching line")
	}
	if operation == "files" || operation == "search" || operation == "discover" {
		fs.StringVar(&o.Glob, "glob", "", "path glob; slashless patterns match basenames; ** crosses directories")
	}
	if operation == "files" {
		fs.StringVar(&o.Type, "type", o.Type, "file, directory, or all")
	}
	if operation == "search" {
		fs.BoolVar(&o.Regex, "regex", false, "interpret query as Go regular expression (line-by-line)")
		fs.BoolVar(&o.IgnoreCase, "ignore-case", false, "case-insensitive search")
	}
	var reads repeated
	if operation == "read" {
		fs.Var(&reads, "file", "PATH or PATH:START:END (inclusive lines); repeat; 0 means start/end")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return &discovery.UsageError{Message: err.Error()}
	}
	if fs.NArg() != 0 {
		return &discovery.UsageError{Message: "unexpected positional arguments; use named flags"}
	}
	o.AllowPaths, o.DenyPaths = allows, denies
	for _, value := range reads {
		parts := strings.Split(value, ":")
		r := discovery.ReadRequest{Path: parts[0]}
		if len(parts) != 1 {
			if len(parts) != 3 {
				return &discovery.UsageError{Message: "file must be PATH or PATH:START:END"}
			}
			var startErr, endErr error
			r.StartLine, startErr = strconv.Atoi(parts[1])
			r.EndLine, endErr = strconv.Atoi(parts[2])
			if startErr != nil || endErr != nil {
				return &discovery.UsageError{Message: "invalid file line range"}
			}
		}
		o.Reads = append(o.Reads, r)
	}
	result, err := discovery.Run(ctx, o)
	if err != nil {
		return err
	}
	return writePayload(*out, result.Payload)
}
