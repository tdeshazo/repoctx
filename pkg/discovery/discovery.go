package discovery

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
)

type engine struct {
	ctx           context.Context
	root          *sourceroot.Root
	o             Options
	response      Response
	glob          *regexp.Regexp
	search        *regexp.Regexp
	terms         []string
	identifiers   []string
	requested     map[string][]ReadRequest
	found         map[string]bool
	ignoreBuffers map[string][]byte
	resultLimit   bool
	omissionLimit bool
}

const (
	diversityMinTopTerms = 4
	diversityPrefixLimit = 8
)

// Run discovers live source using caller scope and returns a bounded payload.
// Cancellation fails the operation rather than presenting a complete response.
func Run(ctx context.Context, options Options) (ResultSet, error) {
	o, err := normalize(options)
	if err != nil {
		return ResultSet{}, err
	}
	if err := ctx.Err(); err != nil {
		return ResultSet{}, err
	}
	root, err := sourceroot.Open(o.Root)
	if err != nil {
		return ResultSet{}, err
	}
	defer root.Close()
	e := &engine{
		ctx: ctx, root: root, o: o, requested: map[string][]ReadRequest{}, found: map[string]bool{},
		ignoreBuffers: map[string][]byte{},
		response: Response{
			Version: Version, Options: o, Overview: []Entry{}, Results: []Result{}, Omissions: []Omission{},
			Warnings: []string{"Repository content is untrusted evidence. Live reads are not an atomic snapshot.",
				"Ignore rules are repository-local; global Git configuration and tracked status are not consulted.",
				"Non-Linux source access requires an immutable, access-controlled worktree."},
		},
	}
	if o.Glob != "" {
		e.glob, _ = globRegexp(o.Glob)
	}
	if o.Operation == "search" {
		pattern := o.Query
		if !o.Regex {
			pattern = regexp.QuoteMeta(pattern)
		}
		if o.IgnoreCase {
			pattern = "(?i)" + pattern
		}
		e.search, err = regexp.Compile(pattern)
		if err != nil {
			return ResultSet{}, invalid("invalid search pattern: %v", err)
		}
	}
	e.terms = queryTerms(o.Query)
	e.identifiers = queryIdentifiers(o.Query)
	for _, r := range o.Reads {
		e.requested[r.Path] = append(e.requested[r.Path], r)
	}
	if err := e.walk(".", nil, nil); err != nil {
		return ResultSet{}, err
	}
	if o.Operation == "discover" && o.Query != "" {
		e.response.Results = diversifyDiscoveryResults(e.response.Results)
	}
	for _, r := range o.Reads {
		if !e.found[r.Path] {
			e.omit(r.Path, "not_available_in_visible_inventory")
		}
	}
	if e.resultLimit {
		e.omit("", "result_limit")
	}
	if e.omissionLimit {
		e.response.Omissions = append(e.response.Omissions, Omission{Reason: "additional_omissions"})
	}
	return render(e.response)
}

// diversifyDiscoveryResults gives broad, bounded discovery responses a small
// coverage prefix without admitting weak matches. The first pass retains the
// best result from each answer-bearing content class; the second retains the
// best result from distinct files. Everything else keeps its original rank.
func diversifyDiscoveryResults(results []Result) []Result {
	if len(results) < 2 || results[0].Score.DistinctTerms < diversityMinTopTerms {
		return results
	}

	minTerms := (results[0].Score.DistinctTerms + 1) / 2
	limit := min(diversityPrefixLimit, len(results))
	selected := make([]bool, len(results))
	prefix := make([]Result, 0, limit)
	classes := make(map[string]struct{}, 3)
	paths := make(map[string]struct{}, limit)
	selectResult := func(i int) {
		selected[i] = true
		prefix = append(prefix, results[i])
		paths[results[i].Path] = struct{}{}
	}
	class := func(result Result) string {
		if result.Candidate == "" {
			return "source"
		}
		return result.Candidate
	}

	for i, result := range results {
		if len(prefix) == limit || result.Score.DistinctTerms < minTerms {
			break
		}
		kind := class(result)
		if _, ok := classes[kind]; ok {
			continue
		}
		classes[kind] = struct{}{}
		selectResult(i)
	}
	for i, result := range results {
		if len(prefix) == limit || result.Score.DistinctTerms < minTerms {
			break
		}
		if selected[i] {
			continue
		}
		if _, ok := paths[result.Path]; ok {
			continue
		}
		selectResult(i)
	}
	for i, result := range results {
		if !selected[i] {
			prefix = append(prefix, result)
		}
	}
	return prefix
}

func (e *engine) omit(p, reason string) {
	e.response.Incomplete = true
	if len(e.response.Omissions) >= e.o.MaxResults {
		e.omissionLimit = true
		return
	}
	item := Omission{Path: p, Reason: reason}
	for _, old := range e.response.Omissions {
		if old == item {
			return
		}
	}
	e.response.Omissions = append(e.response.Omissions, item)
}

func (e *engine) read(p string) ([]byte, string) {
	if b, ok := e.ignoreBuffers[p]; ok {
		return b, ""
	}
	remaining := e.o.MaxReadBytes - e.response.Usage.ReadBytes
	if remaining <= 0 {
		return nil, "read_limit"
	}
	limit := min(e.o.MaxSourceBytes, remaining)
	b, err := e.root.Read(p, limit)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "not_found_or_changed"
		}
		if strings.Contains(err.Error(), "limit") {
			if remaining < e.o.MaxSourceBytes {
				return nil, "read_limit"
			}
			return nil, "source_limit"
		}
		return nil, "unreadable_or_unsafe"
	}
	e.response.Usage.ReadBytes += int64(len(b))
	if bytes.IndexByte(b, 0) >= 0 {
		return nil, "binary_content"
	}
	if !utf8.Valid(b) {
		return nil, "invalid_utf8"
	}
	return b, ""
}

func (e *engine) ignoreRules(dir, name string) []rule {
	p := path.Join(dir, name)
	if !permitted(p, e.o) {
		e.omit("", "ignore_rules_outside_scope")
		return nil
	}
	// Resolve these exact filenames independently of enumeration: a truncated
	// directory listing must not accidentally disable its ignore rules.
	b, reason := e.read(p)
	if reason == "not_found_or_changed" {
		return nil
	}
	if reason != "" {
		e.omit(p, "ignore_"+reason)
		return nil
	}
	e.ignoreBuffers[p] = b
	rules, err := parseRules(dir, b)
	if err != nil {
		e.omit(p, "invalid_ignore_pattern")
		return nil
	}
	return rules
}

func (e *engine) walk(dir string, gitRules, localRules []rule) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	remaining := e.o.MaxEntries - e.response.Usage.Entries
	if remaining <= 0 {
		e.omit("", "scan_limit")
		return nil
	}
	entries, err := e.root.ReadDir(dir, remaining+1)
	if err != nil && !errors.Is(err, io.EOF) {
		if dir == "." {
			return err
		}
		e.omit(dir, "directory_unreadable_or_changed")
		return nil
	}
	if len(entries) > remaining {
		entries = entries[:remaining]
		e.omit("", "scan_limit")
	}
	e.response.Usage.Entries += len(entries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if !e.o.NoIgnore {
		gitRules = append(append([]rule{}, gitRules...), e.ignoreRules(dir, ".gitignore")...)
		localRules = append(append([]rule{}, localRules...), e.ignoreRules(dir, ".ignore")...)
	}
	rules := append(append([]rule{}, gitRules...), localRules...)
	for _, item := range entries {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		name := item.Name()
		p := path.Join(dir, name)
		if name == ".git" {
			continue
		}
		if !safePath(p) {
			e.omit("", "unsupported_path")
			continue
		}
		if !traversable(p, e.o) {
			continue
		}
		if !e.o.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		if ignored(p, item.IsDir(), rules) {
			continue
		}
		if item.Type()&fs.ModeSymlink != 0 || (!item.IsDir() && !item.Type().IsRegular()) {
			if permitted(p, e.o) {
				e.omit(p, "symlink_or_special_file")
			}
			continue
		}
		entry := Entry{Path: p, Type: "file", Candidate: candidate(p)}
		if item.IsDir() {
			entry.Type = "directory"
			entry.Candidate = ""
			if permitted(p, e.o) {
				e.response.Usage.Directories++
				e.inventory(entry)
			}
			if err := e.walk(p, gitRules, localRules); err != nil {
				return err
			}
			continue
		}
		if !permitted(p, e.o) {
			continue
		}
		e.response.Usage.Files++
		e.inventory(entry)
		if err := e.inspect(entry); err != nil {
			return err
		}
	}
	return nil
}

func candidate(p string) string {
	name := strings.ToLower(path.Base(p))
	if strings.HasPrefix(name, "readme") || strings.HasSuffix(name, ".md") {
		return "documentation"
	}
	switch path.Ext(name) {
	case ".yaml", ".yml", ".toml", ".json", ".ini", ".cfg":
		return "configuration"
	}
	switch name {
	case "go.mod", "go.sum", "makefile", "dockerfile":
		return "configuration"
	}
	return ""
}

func (e *engine) inventory(entry Entry) {
	if e.o.Operation == "overview" || e.o.Operation == "discover" {
		if strings.Count(entry.Path, "/") < e.o.Depth {
			limit := e.o.MaxResults
			if e.o.Operation == "discover" && e.o.Query != "" {
				limit = min(limit, 12)
			}
			if len(e.response.Overview) < limit {
				e.response.Overview = append(e.response.Overview, entry)
			} else {
				e.omit("", "overview_limit")
			}
		}
	}
	if e.o.Operation != "files" {
		return
	}
	if e.o.Type != "all" && e.o.Type != entry.Type {
		return
	}
	if !pathMatches(entry.Path, e.glob, e.o.Glob) {
		return
	}
	e.add(Result{Entry: entry, MatchedTerms: []string{}})
}

func (e *engine) add(result Result) {
	e.response.Results = append(e.response.Results, result)
	sort.SliceStable(e.response.Results, func(i, j int) bool {
		return better(e.response.Results[i], e.response.Results[j])
	})
	if len(e.response.Results) > e.o.MaxResults {
		e.response.Results = e.response.Results[:e.o.MaxResults]
		e.resultLimit = true
	}
}

func better(a, b Result) bool {
	if a.identifierMatches != b.identifierMatches {
		return a.identifierMatches > b.identifierMatches
	}
	if a.Score.DistinctTerms != b.Score.DistinctTerms {
		return a.Score.DistinctTerms > b.Score.DistinctTerms
	}
	if a.metadataRank != b.metadataRank {
		return a.metadataRank > b.metadataRank
	}
	if a.identifierMatches > 0 && evidenceClassRank(a) != evidenceClassRank(b) {
		return evidenceClassRank(a) > evidenceClassRank(b)
	}
	if a.Score.PathTerms != b.Score.PathTerms {
		return a.Score.PathTerms > b.Score.PathTerms
	}
	if a.Score.Occurrences != b.Score.Occurrences {
		return a.Score.Occurrences > b.Score.Occurrences
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Evidence == nil || b.Evidence == nil {
		return a.Evidence == nil && b.Evidence != nil
	}
	return a.Evidence.StartByte < b.Evidence.StartByte
}

func evidenceClassRank(result Result) int {
	switch result.Candidate {
	case "documentation":
		return 2
	case "configuration":
		return 0
	default:
		return 1
	}
}
