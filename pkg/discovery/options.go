package discovery

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// DefaultOptions returns CLI defaults. Library callers can start here to select
// zero context lines or a zero-depth overview explicitly.
func DefaultOptions() Options {
	return Options{
		Root: ".", Operation: "discover", Type: "file", Format: "json",
		Depth: 2, ContextLines: 3, MaxBytes: 32768, MaxSourceBytes: 2 << 20,
		MaxReadBytes: 256 << 20, MaxResults: 100, MaxEntries: 100000,
		AllowPaths: []string{}, DenyPaths: []string{}, Reads: []ReadRequest{},
	}
}

func safePath(p string) bool {
	return fs.ValidPath(p) && p != "." && utf8.ValidString(p) && !strings.ContainsAny(p, "\\\x00:")
}

// Validate checks options without accessing repository source.
func Validate(o Options) error {
	_, err := normalize(o)
	return err
}

func normalize(o Options) (Options, error) {
	defaults := DefaultOptions()
	if o.Root == "" {
		o.Root = defaults.Root
	}
	if o.Operation == "" {
		o.Operation = defaults.Operation
	}
	if o.Type == "" {
		o.Type = defaults.Type
	}
	if o.Format == "" {
		o.Format = defaults.Format
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = defaults.MaxBytes
	}
	if o.MaxSourceBytes == 0 {
		o.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if o.MaxReadBytes == 0 {
		o.MaxReadBytes = defaults.MaxReadBytes
	}
	if o.MaxResults == 0 {
		o.MaxResults = defaults.MaxResults
	}
	if o.MaxEntries == 0 {
		o.MaxEntries = defaults.MaxEntries
	}
	if o.MaxBytes < 1 || o.MaxSourceBytes < 1 || o.MaxReadBytes < 1 {
		return o, invalid("byte limits must be positive")
	}
	if o.MaxResults < 1 || o.MaxEntries < 1 || o.MaxEntries > 10000000 {
		return o, invalid("positive result/entry limits required; max-entries must be <= 10000000")
	}
	if o.Depth < 0 || o.ContextLines < 0 {
		return o, invalid("depth and context lines must be nonnegative")
	}
	if o.Format != "json" && o.Format != "markdown" {
		return o, invalid("format must be json or markdown")
	}
	if o.Type != "file" && o.Type != "directory" && o.Type != "all" {
		return o, invalid("type must be file, directory, or all")
	}
	switch o.Operation {
	case "overview", "files":
		if o.Query != "" {
			return o, invalid("query is only supported by search/discover")
		}
	case "search":
		if o.Query == "" {
			return o, invalid("search requires a nonempty query")
		}
		if o.Regex {
			if _, err := regexp.Compile(o.Query); err != nil {
				return o, invalid("invalid regex: %v", err)
			}
		}
	case "read":
		if len(o.Reads) == 0 {
			return o, invalid("read requires at least one file")
		}
		if o.Query != "" {
			return o, invalid("read does not accept a query")
		}
	case "discover":
	default:
		return o, invalid("unknown discovery operation %q", o.Operation)
	}
	if o.Operation != "search" && (o.Regex || o.IgnoreCase) {
		return o, invalid("regex and ignore-case are search options")
	}
	if o.Operation != "read" && len(o.Reads) != 0 {
		return o, invalid("file requests require read")
	}
	if o.Operation == "read" && o.Glob != "" {
		return o, invalid("read does not accept glob filters")
	}
	if o.Glob != "" {
		if _, err := globRegexp(o.Glob); err != nil {
			return o, invalid("invalid glob: %v", err)
		}
	}
	for _, r := range o.Reads {
		if !safePath(r.Path) {
			return o, invalid("unsafe read path %q", r.Path)
		}
		if r.StartLine < 0 || r.EndLine < 0 || (r.EndLine > 0 && r.EndLine < r.StartLine) {
			return o, invalid("invalid line range for %q", r.Path)
		}
	}
	o.AllowPaths = append([]string{}, o.AllowPaths...)
	o.DenyPaths = append([]string{}, o.DenyPaths...)
	o.Reads = append([]ReadRequest{}, o.Reads...)
	for _, prefixes := range [][]string{o.AllowPaths, o.DenyPaths} {
		for i, p := range prefixes {
			p = strings.TrimSuffix(p, "/")
			if !safePath(p) || strings.ContainsAny(p, "*?[") {
				return o, invalid("invalid scope prefix %q", p)
			}
			prefixes[i] = p
		}
	}
	abs, err := filepath.Abs(o.Root)
	if err != nil {
		return o, err
	}
	o.Root = abs
	return o, nil
}
