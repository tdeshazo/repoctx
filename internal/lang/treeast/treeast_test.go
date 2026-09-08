package treeast

import (
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestParseLanguagesAndSourceLinkedLowering(t *testing.T) {
	cases := []struct {
		name string
		lang Language
		src  string
		want []string
		edge ir.EdgeKind
	}{
		{"python", Python, "import pathlib\nclass Worker:\n    def run(self):\n        return pathlib.Path('.')\n", []string{"Worker", "run"}, ir.EdgeCalls},
		{"html", HTML, "<main id=\"app\" href=\"/home\"><h1>Hello</h1></main>", []string{"main", "h1"}, ir.EdgeImports},
		{"css", CSS, "@import url(\"theme.css\"); .app { color: red; }", []string{"app", "color"}, ir.EdgeImports},
		{"javascript", JavaScript, "import x from 'x'; class Worker { run() { return ping(); } }", []string{"Worker", "run"}, ir.EdgeCalls},
		{"typescript", TypeScript, "import { helper } from './helper'; interface Worker { run(): string } type Result = string | number; export function run(value: Result): string { return helper(value); }", []string{"Worker", "Result", "run"}, ir.EdgeCalls},
		{"tsx", TSX, "import React from 'react'; interface Props { name: string } export function App(props: Props) { return <section>{render(props.name)}</section>; }", []string{"Props", "App"}, ir.EdgeCalls},
		{"markdown", Markdown, "# Overview\n\nSee [guide](docs/guide.md).\n", []string{"Overview"}, ir.EdgeReferences},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := ir.NewStrings()
			got, err := Parse(tc.lang, []byte(tc.src), st)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Nodes) < 2 || len(got.Roots) != 1 || got.Roots[0] != 0 {
				t.Fatalf("nodes=%d roots=%v", len(got.Nodes), got.Roots)
			}
			for i, n := range got.Nodes {
				if n.Span.SL < 1 || n.Span.EL < n.Span.SL || n.Span.EC < n.Span.SC && n.Span.EL == n.Span.SL {
					t.Fatalf("node %d invalid span: %#v", i, n.Span)
				}
				for _, child := range n.Children {
					if child <= i || child >= len(got.Nodes) {
						t.Fatalf("node %d invalid child %d", i, child)
					}
				}
			}
			for _, want := range tc.want {
				found := false
				for _, symbol := range got.Symbols {
					if symbol.Name == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing source-linked symbol %q: %#v", want, got.Symbols)
				}
			}
			if tc.edge != ir.EdgeUnknown {
				found := false
				for _, edge := range got.Edges {
					if edge.Kind == tc.edge && edge.Text != "" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing %v edge: %#v", tc.edge, got.Edges)
				}
			}
		})
	}
}

func TestMalformedInputIsRejectedWithBoundedDiagnostic(t *testing.T) {
	cases := []struct {
		lang Language
		src  string
	}{
		{Python, "def broken(:\n    pass\n"},
		{HTML, "<main><span>"},
		{CSS, ".broken { color: ;"},
		{JavaScript, "function broken( {"},
		{TypeScript, "interface Broken { value: string;"},
		{TSX, "export function Broken() { return <main>; }"},
	}
	for _, tc := range cases {
		_, err := Parse(tc.lang, []byte(tc.src), ir.NewStrings())
		if err == nil {
			t.Errorf("%s malformed input unexpectedly parsed", languageName(tc.lang))
		} else if len(err.Error()) > 512 || !strings.Contains(err.Error(), "tree-sitter") {
			t.Errorf("%s diagnostic is not bounded/descriptive: %v", languageName(tc.lang), err)
		}
	}
}

func TestTypeScriptImportsAndCallsAreSourceLinked(t *testing.T) {
	src := []byte("import { helper } from './helper'; export function run() { return helper(); }\n")
	result, err := Parse(TypeScript, src, ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	var imported, called bool
	for _, edge := range result.Edges {
		switch {
		case edge.Kind == ir.EdgeImports && edge.Text == "./helper":
			imported = true
		case edge.Kind == ir.EdgeCalls && edge.Text == "helper":
			called = true
		}
	}
	if !imported || !called {
		t.Fatalf("source-linked edges: imported=%v called=%v edges=%#v", imported, called, result.Edges)
	}
}

func TestTypeScriptImportEqualsIsSourceLinked(t *testing.T) {
	result, err := Parse(TypeScript, []byte(`import pkg = require("pkg");`), ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range result.Edges {
		if edge.Kind == ir.EdgeImports && edge.Text == "pkg" {
			return
		}
	}
	t.Fatalf("missing import-equals edge: %#v", result.Edges)
}

func TestNonNullCallsAreSourceLinked(t *testing.T) {
	cases := []struct {
		name string
		lang Language
		src  string
	}{
		{"typescript", TypeScript, "declare const handler: () => void; handler!();"},
		{"tsx", TSX, "declare const handler: () => void; const App = () => <>{handler!()}</>;"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Parse(tc.lang, []byte(tc.src), ir.NewStrings())
			if err != nil {
				t.Fatal(err)
			}
			for _, edge := range result.Edges {
				if edge.Kind == ir.EdgeCalls && edge.Text == "handler" {
					return
				}
			}
			t.Fatalf("missing non-null call edge: %#v", result.Edges)
		})
	}
}

func TestInvalidUTF8RejectedBeforeNativeParse(t *testing.T) {
	if _, err := Parse(Python, []byte{'d', 'e', 'f', ' ', 0xff}, ir.NewStrings()); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestMarkdownFencedCodeIsRawOnly(t *testing.T) {
	src := []byte("# Notes\n\n```go\nfunc Hidden() { helper() }\nimport \"hidden\"\n```\n")
	result, err := Parse(Markdown, src, ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range result.Symbols {
		if symbol.Name == "Hidden" || symbol.Name == "helper" || symbol.Name == "hidden" {
			t.Fatalf("fenced code was lowered as a symbol: %#v", result.Symbols)
		}
	}
	for _, edge := range result.Edges {
		if edge.Kind == ir.EdgeCalls || edge.Kind == ir.EdgeImports {
			t.Fatalf("fenced code produced executable-language edge: %#v", result.Edges)
		}
	}
	var raw bool
	for _, node := range result.Nodes {
		if node.Kind > 0 {
			raw = true
		}
	}
	if !raw {
		t.Fatal("markdown AST is empty")
	}
}

func TestMarkdownLinkDestinationPreservesRelativePrefix(t *testing.T) {
	result, err := Parse(Markdown, []byte("# Links\n\nSee [guide](./docs/guide.md).\n"), ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range result.Edges {
		if edge.Kind == ir.EdgeReferences && edge.Text == "./docs/guide.md" {
			return
		}
	}
	t.Fatalf("relative destination was not preserved: %#v", result.Edges)
}

func TestMarkdownAutolinksProduceNormalizedReferences(t *testing.T) {
	result, err := Parse(Markdown, []byte("See <https://example.test/a> and <person@example.test>.\n"), ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{"https://example.test/a", "person@example.test"} {
		if !hasMarkdownReferenceAtLine(result, destination, 1) {
			t.Fatalf("autolink %q missing from %#v", destination, result.Edges)
		}
	}
}

func TestMarkdownReferenceLinksResolveDefinitions(t *testing.T) {
	src := []byte("[full text][Full Label] [collapsed][] [shortcut] ![image alt][Image Label]\n\n[full label]: https://example.test/full\n[collapsed]: ./docs/collapsed.md\n[shortcut]: /shortcut\n[image label]: assets/image.png\n")
	result, err := Parse(Markdown, src, ir.NewStrings())
	if err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{"https://example.test/full", "./docs/collapsed.md", "/shortcut", "assets/image.png"} {
		if !hasMarkdownReferenceAtLine(result, destination, 1) {
			t.Fatalf("reference destination %q missing from %#v", destination, result.Edges)
		}
	}
}

func hasMarkdownReferenceAtLine(result Result, destination string, line int) bool {
	for _, edge := range result.Edges {
		if edge.Kind == ir.EdgeReferences && edge.Text == destination && edge.Node >= 0 && edge.Node < len(result.Nodes) && result.Nodes[edge.Node].Span.SL == line {
			return true
		}
	}
	return false
}

func TestParseRepeatedlyClosesNativeResources(t *testing.T) {
	const iterations = 1_000
	for i := 0; i < iterations; i++ {
		result, err := Parse(Python, []byte("def worker():\n    return 1\n"), ir.NewStrings())
		if err != nil {
			t.Fatalf("parse %d: %v", i, err)
		}
		if len(result.Nodes) == 0 {
			t.Fatalf("parse %d returned no nodes", i)
		}
	}
}
