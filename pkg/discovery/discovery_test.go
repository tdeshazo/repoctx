package discovery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, text := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func run(t *testing.T, o Options) ResultSet {
	t.Helper()
	r, err := Run(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Payload) > o.MaxBytes {
		t.Fatal("payload exceeded budget")
	}
	return r
}

func paths(r ResultSet) []string {
	ps := []string{}
	for _, item := range r.Response.Results {
		ps = append(ps, item.Path)
	}
	return ps
}

func hasReason(r ResultSet, reason string) bool {
	for _, omission := range r.Response.Omissions {
		if omission.Reason == reason {
			return true
		}
	}
	return false
}

func TestVisibilityAndIgnorePrecedence(t *testing.T) {
	o := DefaultOptions()
	o.Operation = "files"
	o.Root = fixture(t, map[string]string{
		".gitignore": "*.tmp\ncache/\n/root-only.txt\n**/out/*.txt\n!keep.tmp\n",
		".ignore":    "keep.tmp\n!visible.tmp\n",
		"keep.tmp":   "", "visible.tmp": "", "a.tmp": "", "root-only.txt": "",
		"nested/root-only.txt": "", "cache/keep.txt": "", "out/a.txt": "",
		"nested/out/b.txt": "", "nested/.gitignore": "!child.tmp\n!keep.tmp\n",
		"nested/child.tmp": "", "nested/keep.tmp": "", "config.yaml": "a: b",
		"go.mod": "module example.test", ".hidden": "", ".git/config": "private",
	})
	r := run(t, o)
	want := []string{"config.yaml", "go.mod", "nested/child.tmp", "nested/root-only.txt", "visible.tmp"}
	if !reflect.DeepEqual(paths(r), want) {
		t.Fatalf("got %v want %v", paths(r), want)
	}
	o.Hidden, o.NoIgnore = true, true
	o.DenyPaths = []string{"cache", "nested"}
	r = run(t, o)
	for _, p := range paths(r) {
		if strings.HasPrefix(p, "cache/") || strings.HasPrefix(p, "nested/") || strings.HasPrefix(p, ".git/") {
			t.Fatalf("denied path returned: %s", p)
		}
	}
	if !strings.Contains(strings.Join(paths(r), ","), ".hidden") {
		t.Fatal("hidden override failed")
	}
}

func TestIgnorePatterns(t *testing.T) {
	cases := []struct {
		pattern, path   string
		directory, want bool
	}{
		{"/a", "nested/a", false, false}, {"a", "nested/a", false, true},
		{"a/", "a", false, false}, {"a/", "a", true, true},
		{"a/**/b", "a/b", false, true}, {"a/**/b", "a/x/y/b", false, true},
		{"**/é?.[ch]", "pkg/éx.c", false, true}, {"a[!b]", "ac", false, true},
		{"\\#tag", "#tag", false, true}, {"\\!tag", "!tag", false, true},
		{"space\\ ", "space ", false, true}, {"trim  ", "trim", false, true},
	}
	for _, tc := range cases {
		rules, err := parseRules(".", []byte(tc.pattern))
		if err != nil {
			t.Fatal(err)
		}
		if got := ignored(tc.path, tc.directory, rules); got != tc.want {
			t.Errorf("%q matching %q got %v", tc.pattern, tc.path, got)
		}
	}
}

func TestExactEvidenceAndBatchedReads(t *testing.T) {
	text := "title\r\nα one\r\nβ two\r\nlast"
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"doc.txt": text})
	o.Operation = "read"
	o.Reads = []ReadRequest{{Path: "doc.txt", StartLine: 2, EndLine: 3}, {Path: "doc.txt", StartLine: 3, EndLine: 4}}
	r := run(t, o)
	if len(r.Response.Results) != 1 {
		t.Fatalf("overlap not merged: %+v", r.Response.Results)
	}
	ev := r.Response.Results[0].Evidence
	if ev.Text != "α one\r\nβ two\r\nlast" || ev.StartByte != 7 || ev.EndByte != len(text) {
		t.Fatalf("wrong exact evidence: %+v", ev)
	}
	hash := sha256.Sum256([]byte(text))
	if ev.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("incorrect hash")
	}
	if r.Response.Usage.ReadBytes != int64(len(text)) {
		t.Fatal("batched reads reread source")
	}
	if err := os.WriteFile(filepath.Join(o.Root, "doc.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	o.Reads = []ReadRequest{{Path: "doc.txt"}}
	changed := run(t, o)
	if changed.Response.Results[0].Evidence.Text != "new" {
		t.Fatal("served stale content")
	}
}

func TestReadRangeEOFRecovery(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"doc.txt":   "one\ntwo\nthree",
		"empty.txt": "",
		"short.txt": "one",
	})
	o.Operation = "read"
	o.Reads = []ReadRequest{
		{Path: "doc.txt", StartLine: 2, EndLine: 9},
		{Path: "empty.txt", StartLine: 1, EndLine: 2},
		{Path: "short.txt", StartLine: 4},
	}
	_, err := Run(context.Background(), o)
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("expected usage error, got %v", err)
	}
	for _, want := range []string{
		"requested end line 9 for 'doc.txt:2:9' exceeds observed file \"doc.txt\" (3 lines)",
		"-file 'doc.txt:2:0' to read through EOF",
		"requested end line 2 for 'empty.txt:1:2' exceeds observed file \"empty.txt\" (1 line)",
		"-file 'empty.txt:1:0' to read through EOF",
		"requested start line 4 for 'short.txt:4:0' exceeds observed file \"short.txt\" (1 line)",
		"choose a start line from 1 through 1",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing range recovery detail %q: %v", want, err)
		}
	}

	o.Reads = []ReadRequest{{Path: "doc.txt", StartLine: 2, EndLine: 0}}
	r := run(t, o)
	if len(r.Response.Results) != 1 {
		t.Fatalf("unexpected EOF recovery result: %+v", r.Response.Results)
	}
	evidence := r.Response.Results[0].Evidence
	if evidence.Text != "two\nthree" || evidence.StartLine != 2 || evidence.EndLine != 3 {
		t.Fatalf("EOF recovery did not preserve exact requested bytes: %+v", evidence)
	}

	o.Reads = []ReadRequest{{Path: "empty.txt", StartLine: 1, EndLine: 0}}
	r = run(t, o)
	if len(r.Response.Results) != 1 {
		t.Fatalf("unexpected empty-file EOF result: %+v", r.Response.Results)
	}
	evidence = r.Response.Results[0].Evidence
	if evidence.Text != "" || evidence.StartLine != 1 || evidence.EndLine != 1 {
		t.Fatalf("empty-file EOF read changed its observed span: %+v", evidence)
	}
}

func TestDiscoverAnswerBearingBodies(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"docs/release.md": "# Release\n\nThe release gate requires reproducible output.\n",
		"settings.yaml":   "release:\n  reproducible: true\n",
		"source.go":       "package main\nfunc unrelated() {}\n",
	})
	o.Query = "release gate reproducible"
	r := run(t, o)
	if len(r.Response.Results) != 2 || len(r.Response.Overview) == 0 {
		t.Fatalf("missing combined results: %+v", r.Response)
	}
	if r.Response.Results[0].Path != "docs/release.md" {
		t.Fatal("ranking did not prioritize distinct terms")
	}
	if !strings.Contains(r.Response.Results[0].Evidence.Text, "requires reproducible output") {
		t.Fatal("body missing")
	}
	if !strings.Contains(r.Response.Results[1].Evidence.Text, "reproducible: true") {
		t.Fatal("config missing")
	}
	if r.Response.Results[0].Score.DistinctTerms != 3 {
		t.Fatal("ranking not explained")
	}
	r2 := run(t, o)
	if !bytes.Equal(r.Payload, r2.Payload) {
		t.Fatal("nondeterministic complete response")
	}
}

func TestDiscoverPrefersCanonicalLeavesOverGeneratedRoutes(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"docs/index.md": "---\nschema: rcx.navigation/v1\nid: rcx.topic.retry\nkind: map\norigin: generated\nstatus: active\nauthority: generated\n---\n\n# Retry topic\n\nUse the retry configuration route for the canonical guidance.\n",
		"docs/retry.md": "---\nschema: rcx.document/v1\nid: rcx.retry.guide\nkind: architecture\norigin: report\nstatus: active\nauthority: advisory\n---\n\n# Retry configuration\n\nCanonical retry configuration guidance.\n",
	})
	o.Query, o.MaxResults, o.ContextLines = "retry configuration", 1, 0
	r := run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Results[0].Path != "docs/retry.md" {
		t.Fatalf("generated route consumed bounded result: %+v", r.Response.Results)
	}
	if r.Response.Results[0].Evidence == nil || r.Response.Results[0].Evidence.Text != "# Retry configuration\n" {
		t.Fatalf("canonical evidence changed: %+v", r.Response.Results[0].Evidence)
	}
	if strings.Contains(string(r.Payload), "metadataRank") {
		t.Fatal("internal metadata rank leaked into the discovery contract")
	}
}

func TestDiscoverMetadataDoesNotOverrideStrongerLexicalEvidence(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"docs/index.md": "---\nschema: rcx.navigation/v1\nkind: map\norigin: generated\nstatus: active\n---\n\nretry configuration\n",
		"docs/retry.md": "---\nschema: rcx.document/v1\nkind: architecture\norigin: report\nstatus: active\n---\n\nretry\n",
		"pkg/retry.go":  "package retry\n// retry configuration reproducible\n",
	})
	o.Query, o.MaxResults, o.ContextLines = "retry configuration reproducible", 1, 0
	r := run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Results[0].Path != "pkg/retry.go" {
		t.Fatalf("metadata rank overrode stronger lexical evidence: %+v", r.Response.Results)
	}
}

func TestMetadataRankUsesDeclaredFrontmatterOnly(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{
			name: "generated navigation",
			text: "---\nschema: rcx.navigation/v1\nkind: map\norigin: generated\nstatus: active\n---\nbody",
			want: -3,
		},
		{
			name: "active canonical leaf",
			text: "---\nschema: rcx.document/v1\nkind: architecture\norigin: report\nstatus: active\n---\nbody",
			want: 3,
		},
		{
			name: "deprecated canonical leaf",
			text: "---\nschema: rcx.document/v1\nkind: architecture\norigin: report\nstatus: deprecated\n---\nbody",
			want: -2,
		},
		{
			name: "prose is not metadata",
			text: "# origin: generated\n# kind: map\nbody",
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := metadataRank([]byte(tc.text)); got != tc.want {
				t.Fatalf("metadata rank = %d, want %d", got, tc.want)
			}
		})
	}
}

func BenchmarkMetadataRankNonFrontmatter(b *testing.B) {
	data := append([]byte("ordinary source\n"), bytes.Repeat([]byte("retry configuration evidence\n"), 1<<16)...)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if got := metadataRank(data); got != 0 {
			b.Fatalf("metadata rank = %d, want 0", got)
		}
	}
}

func TestDiscoverDiversifiesBroadRelevantResults(t *testing.T) {
	results := []Result{
		{Entry: Entry{Path: "docs/a.md", Candidate: "documentation"}, Score: Score{DistinctTerms: 8}},
		{Entry: Entry{Path: "docs/a.md", Candidate: "documentation"}, Score: Score{DistinctTerms: 7}},
		{Entry: Entry{Path: "docs/b.md", Candidate: "documentation"}, Score: Score{DistinctTerms: 6}},
		{Entry: Entry{Path: "pkg/answer.go"}, Score: Score{DistinctTerms: 5}},
		{Entry: Entry{Path: "settings.yaml", Candidate: "configuration"}, Score: Score{DistinctTerms: 4}},
		{Entry: Entry{Path: "pkg/noise.go"}, Score: Score{DistinctTerms: 3}},
	}

	got := diversifyDiscoveryResults(results)
	want := []string{"docs/a.md", "pkg/answer.go", "settings.yaml", "docs/b.md", "docs/a.md", "pkg/noise.go"}
	if !reflect.DeepEqual(resultPaths(got), want) {
		t.Fatalf("diversified paths = %v, want %v", resultPaths(got), want)
	}
	if got[0].Score.DistinctTerms != 8 || got[len(got)-1].Score.DistinctTerms != 3 {
		t.Fatal("diversity admitted weak evidence into the coverage prefix")
	}

	shortQuery := append([]Result(nil), results...)
	shortQuery[0].Score.DistinctTerms = diversityMinTopTerms - 1
	if got := diversifyDiscoveryResults(shortQuery); !reflect.DeepEqual(got, shortQuery) {
		t.Fatal("short-query ranking changed")
	}
}

func TestDiscoverBoundedIdentifierRetrieval(t *testing.T) {
	filler := func(term string, lines int) string {
		return strings.Repeat(term+" "+strings.Repeat("x", 160)+"\n", lines)
	}
	tests := []struct {
		name, query, decisive string
		maxBytes              int
		files                 map[string]string
	}{
		{
			name: "exact identifier outranks split generated noise", query: "M4-07",
			decisive: "M4-07 requires an atomic checkpoint.", maxBytes: 5000,
			files: map[string]string{
				"ROADMAP.md":     filler("unrelated roadmap context", 5) + "- [ ] M4-07 requires an atomic checkpoint.\n" + filler("later roadmap context", 5),
				"generated.json": filler("hash m4 payload 07", 40),
			},
		},
		{
			name: "answer window fits beside response metadata", query: "M4-08 persisted evidence evaluation",
			decisive: "M4-08 evaluates persisted evidence after compaction.", maxBytes: 8000,
			files: map[string]string{
				"ROADMAP.md": filler("persisted evidence evaluation", 30) +
					"- [ ] M4-08 evaluates persisted evidence after compaction.\n" +
					filler("persisted evidence evaluation", 30),
			},
		},
		{
			name: "later actionable identifier window is retained", query: "M4-09 context failure diagnosis",
			decisive: "M4-09 diagnose context failures and retain supporting traces.", maxBytes: 12000,
			files: map[string]string{
				"ROADMAP.md": "Planning input mentions M4-09 context failure diagnosis.\n" +
					filler("context failure diagnosis", 45) +
					"- [ ] M4-09 diagnose context failures and retain supporting traces.\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.Root, o.Query, o.MaxBytes, o.Depth = fixture(t, tc.files), tc.query, tc.maxBytes, 0
			r := run(t, o)
			var found bool
			for _, result := range r.Response.Results {
				if result.Evidence == nil || !strings.Contains(result.Evidence.Text, tc.decisive) {
					continue
				}
				found = true
				source, err := os.ReadFile(filepath.Join(o.Root, filepath.FromSlash(result.Path)))
				if err != nil {
					t.Fatal(err)
				}
				if got := string(source[result.Evidence.StartByte:result.Evidence.EndByte]); got != result.Evidence.Text {
					t.Fatalf("evidence is not the exact source span: %q", got)
				}
				if result.Evidence.StartByte == 0 && result.Evidence.EndByte == len(source) {
					t.Fatal("bounded result did not identify an excerpt")
				}
			}
			if !found {
				t.Fatalf("decisive evidence was not retained: %+v", r.Response.Results)
			}
		})
	}

	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"ROADMAP.md": "- [ ] M4-10 nearby requirement\n"})
	o.Query, o.MaxBytes, o.Depth = "M4-10 nearby requirement", 5000, 0
	if r := run(t, o); len(r.Response.Results) != 1 || !strings.Contains(r.Response.Results[0].Evidence.Text, "nearby requirement") {
		t.Fatalf("nearby identifier query regressed: %+v", r.Response.Results)
	}
	o.Query = "Z9-99 quasar nebula"
	if r := run(t, o); len(r.Response.Results) != 0 || r.Response.Incomplete {
		t.Fatalf("no-answer query should be complete and empty: %+v", r.Response)
	}
}

func resultPaths(results []Result) []string {
	paths := make([]string, 0, len(results))
	for _, result := range results {
		paths = append(paths, result.Path)
	}
	return paths
}

func TestSearchLiteralRegexAndEmpty(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"a.txt": "Hello.*\nhello world\n", "b.txt": "other"})
	o.Operation, o.Query, o.ContextLines = "search", "Hello.*", 0
	r := run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Results[0].Evidence.Text != "Hello.*\n" {
		t.Fatal("literal mismatch")
	}
	o.Regex, o.IgnoreCase = true, true
	o.Query = "^hello"
	r = run(t, o)
	if r.Response.Results[0].Evidence.Text != "Hello.*\nhello world\n" {
		t.Fatal("regex/case handling")
	}
	o.Query = "nonexistent"
	r = run(t, o)
	if len(r.Response.Results) != 0 || r.Response.Incomplete {
		t.Fatal("empty search is not a failure")
	}
}

func TestScopeAndUnsafeSources(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		".gitignore": "allowed/\n", "allowed/a.txt": "needle", "private/secret.txt": "secret",
	})
	o.AllowPaths = []string{"allowed"}
	o.Operation, o.Query = "search", "needle"
	outside := fixture(t, map[string]string{"escape.txt": "needle"})
	if err := os.Symlink(outside, filepath.Join(o.Root, "allowed", "link")); err != nil {
		t.Fatal(err)
	}
	r := run(t, o)
	if !reflect.DeepEqual(paths(r), []string{"allowed/a.txt"}) {
		t.Fatalf("scope failed: %v", paths(r))
	}
	if !hasReason(r, "ignore_rules_outside_scope") || !hasReason(r, "symlink_or_special_file") {
		t.Fatal("missing limitations")
	}
	if strings.Contains(string(r.Payload), "secret.txt") {
		t.Fatal("denied filename leaked")
	}
	o.Operation, o.Query = "read", ""
	o.Reads = []ReadRequest{{Path: "allowed/link/escape.txt"}, {Path: "private/secret.txt"}}
	r = run(t, o)
	if len(r.Response.Results) != 0 {
		t.Fatal("unsafe direct read")
	}
}

func TestLimitsAndUnavailableContent(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"binary.txt": "a\x00b", "invalid.txt": "\xff", "large.txt": strings.Repeat("x", 100), "small.txt": "ok",
	})
	o.Operation, o.Query, o.MaxSourceBytes = "search", "ok", 32
	r := run(t, o)
	for _, reason := range []string{"binary_content", "invalid_utf8", "source_limit"} {
		if !hasReason(r, reason) {
			t.Errorf("missing %s", reason)
		}
	}
	o.MaxReadBytes = 1
	r = run(t, o)
	if r.Response.Usage.ReadBytes > 1 || !r.Response.Incomplete {
		t.Fatal("read budget")
	}
	o.Operation, o.Query, o.MaxEntries = "files", "", 1
	r = run(t, o)
	if r.Response.Usage.Entries > 1 || !hasReason(r, "scan_limit") {
		t.Fatal("scan bound")
	}
	o.MaxEntries, o.MaxResults = 100, 1
	r = run(t, o)
	if len(r.Response.Results) != 1 || !hasReason(r, "result_limit") {
		t.Fatal("result bound")
	}
}

func TestFinalPayloadBounds(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"huge.txt": strings.Repeat("\"α<&\t", 10000)})
	o.Operation, o.Reads = "read", []ReadRequest{{Path: "huge.txt"}}
	for _, format := range []string{"json", "markdown"} {
		o.Format, o.MaxBytes = format, 4096
		r := run(t, o)
		if len(r.Response.Results) != 0 || !hasReason(r, "output_limit") {
			t.Fatal("oversized record not explicitly omitted")
		}
		if format == "json" && !json.Valid(r.Payload) {
			t.Fatal("truncated JSON")
		}
	}
	o.MaxBytes = 1
	if _, err := Run(context.Background(), o); err == nil {
		t.Fatal("metadata should not fit")
	}
}

func TestValidationAndCancellation(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"a": "text"})
	for _, request := range []ReadRequest{
		{Path: "../a"}, {Path: "/a"}, {Path: "a", StartLine: 2, EndLine: 1},
		{Path: "a", StartLine: 10},
	} {
		o.Operation, o.Reads = "read", []ReadRequest{request}
		_, err := Run(context.Background(), o)
		var usage *UsageError
		if !errors.As(err, &usage) {
			t.Errorf("expected usage error for %+v: %v", request, err)
		}
	}
	o = DefaultOptions()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, o); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDirectoriesGlobsAndRoot(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"src/é.py": "", "tests/a.txt": ""})
	o.Operation, o.Type, o.Glob = "files", "directory", "test*"
	if got := paths(run(t, o)); !reflect.DeepEqual(got, []string{"tests"}) {
		t.Fatal(got)
	}
	o.Type, o.Glob = "file", "**/é.py"
	if got := paths(run(t, o)); !reflect.DeepEqual(got, []string{"src/é.py"}) {
		t.Fatal(got)
	}
	o.Root, o.Glob = filepath.Join(o.Root, "src"), ""
	if got := paths(run(t, o)); !reflect.DeepEqual(got, []string{"é.py"}) {
		t.Fatal("root expanded", got)
	}
}

func TestUnreadableAndEmptyFile(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"empty": "", "denied": "do not read"})
	denied := filepath.Join(o.Root, "denied")
	if err := os.Chmod(denied, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0600) })
	o.Operation, o.Reads = "read", []ReadRequest{{Path: "empty"}, {Path: "denied"}}
	r := run(t, o)
	if len(r.Response.Results) == 2 {
		t.Skip("execution identity bypasses file permissions")
	}
	if !hasReason(r, "unreadable_or_unsafe") {
		t.Fatal("missing permission omission")
	}
	if len(r.Response.Results) != 1 || r.Response.Results[0].Evidence.Text != "" {
		t.Fatal("empty file was not preserved")
	}
}

func TestMarkdownEvidenceParity(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"a.txt": "```\n# untrusted\nα<&\r\n"})
	o.Operation, o.Reads = "read", []ReadRequest{{Path: "a.txt"}}
	jsonResult := run(t, o)
	o.Format = "markdown"
	markdown := run(t, o)
	if !reflect.DeepEqual(jsonResult.Response.Results, markdown.Response.Results) {
		t.Fatal("evidence differs across formats")
	}
	lines := strings.Split(string(markdown.Payload), "\n")
	encoded := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "    ") {
			encoded = append(encoded, line[4:])
		}
	}
	var decoded Response
	if err := json.Unmarshal([]byte(strings.Join(encoded, "\n")), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, markdown.Response) {
		t.Fatal("Markdown did not round-trip contract")
	}
}

func TestCancellationDuringScan(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"a.txt": "line\nline\nline"})
	ctx := &cancelAfterChecks{Context: context.Background(), remaining: 3}
	if _, err := Run(ctx, o); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan ignored cancellation: %v", err)
	}
}

func TestVisibleIgnoreFileReusesObservedBuffer(t *testing.T) {
	o := DefaultOptions()
	text := "# needle\n*.tmp\n"
	o.Root = fixture(t, map[string]string{".gitignore": text})
	o.Hidden, o.Operation, o.Query = true, "search", "needle"
	r := run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Usage.ReadBytes != int64(len(text)) {
		t.Fatal("ignore file was read again during content search")
	}
}

func TestScanLimitDoesNotDisableIgnoreRules(t *testing.T) {
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{"a.txt": "private", "b.txt": "private", ".gitignore": "*.txt\n"})
	o.Operation, o.MaxEntries = "files", 1
	r := run(t, o)
	if len(r.Response.Results) != 0 || !hasReason(r, "scan_limit") {
		t.Fatal("truncated enumeration disabled ignore rules")
	}
}

type cancelAfterChecks struct {
	context.Context
	remaining int
}

func (c *cancelAfterChecks) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}
