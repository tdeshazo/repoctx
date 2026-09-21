package agentctx

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compiler"
)

func TestM1DocumentUnitsAndContainment(t *testing.T) {
	doc := "Intro α.\r\n\r\n# Repeated\r\nParagraph body.\r\n\r\n## Nested\r\n- [ ] First criterion\r\n\r\n" +
		"| A | B |\r\n| --- | --- |\r\n| yes | no |\r\n\r\n```go\r\n# not a heading\r\nfunc Fake() {}\r\n```\r\n\r\n# Repeated\r\nLast."
	root, r := compileFixture(t, map[string]string{"doc.md": doc, "plain.md": "No heading.\n\nAnother paragraph.\n"})
	o := baseOptions(root)
	if err := normalize(&o); err != nil {
		t.Fatal(err)
	}
	sources, err := loadSources(r, o)
	if err != nil {
		t.Fatal(err)
	}
	units, err := retrievalUnits(r, sources)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]retrievalUnit{}
	kinds := map[string]int{}
	sections := []retrievalUnit{}
	for _, u := range units {
		if _, exists := byID[u.ID]; exists {
			t.Fatal("colliding unit IDs")
		}
		byID[u.ID] = u
		kinds[u.Kind]++
		if u.Kind == "section" {
			sections = append(sections, u)
		}
		if u.end > len(sources[u.file].data) {
			t.Fatal("invalid unit extent")
		}
	}
	for _, kind := range []string{"document", "section", "paragraph", "checklist_item", "table", "code_block"} {
		if kinds[kind] == 0 {
			t.Errorf("missing %s", kind)
		}
	}
	if len(sections) != 3 {
		t.Fatalf("fence promoted to heading: %d sections", len(sections))
	}
	boundary := strings.LastIndex(doc, "# Repeated")
	if sections[0].end != boundary || sections[1].end != boundary || sections[2].end != len(doc) {
		t.Fatal("wrong section boundaries")
	}
	for _, section := range sections {
		if section.Heading == nil || section.Content == nil {
			t.Fatal("heading and content spans not separated")
		}
		if section.Content.StartLine != section.Heading.EndLine || section.Content.StartByteColumn != section.Heading.EndByteColumn {
			t.Fatal("section content overlaps heading")
		}
	}
	for _, u := range units {
		if u.Parent == "" {
			if u.Kind != "document" {
				t.Fatal("orphan unit")
			}
			continue
		}
		parent := byID[u.Parent]
		if parent.file != u.file || parent.start > u.start || parent.end < u.end || u.Parent == u.ID {
			t.Fatal("bad containment")
		}
	}
	for _, s := range r.Symbols {
		if strings.Contains(r.String(s.Name), "Fake") {
			t.Fatal("fence parsed as Go")
		}
	}
}

func TestM1BodyOnlyAndExplicitUnit(t *testing.T) {
	root, r := compileFixture(t, map[string]string{"doc.md": "# General\n\nThe uncommon zephyr contract is enforced.\n"})
	o := baseOptions(root)
	o.Query = "zephyr"
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bundle.Units) == 0 {
		t.Fatal("body did not seed retrieval")
	}
	assertEvidence(t, root, res.Bundle)
	o.Query, o.Units = "", []string{res.Bundle.Units[0].ID}
	o.ExpectedSnapshot = res.Bundle.Snapshot.ID
	explicit, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if explicit.Bundle.Units[0].Reason.Strategy != "explicit_unit" {
		t.Fatal("unit not explicit")
	}
	o.DenyPaths = []string{"doc.md"}
	if _, err := Build(r, o); err == nil {
		t.Fatal("denied unit accepted")
	}
	o.DenyPaths = nil
	o.MaxBytes = 1500
	if _, err := Build(r, o); err == nil {
		t.Fatal("required unit silently dropped")
	}
	o.MaxBytes, o.Units, o.Query = 20000, nil, "missingxyz"
	if _, err := Build(r, o); err == nil {
		t.Fatal("no-match substituted unrelated context")
	}
}

func TestM1DecisiveBranchAtEnd(t *testing.T) {
	code := "package p\nfunc Evaluate() string {\n" + strings.Repeat(" println(\"padding padding padding\")\n", 600) + " return \"zephyr rejected\"\n}\n"
	root, r := compileFixture(t, map[string]string{"main.go": code})
	o := baseOptions(root)
	o.Query, o.MaxBytes = "zephyr rejected", 7000
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	found, lead, aligned := false, false, false
	for _, e := range res.Bundle.Evidence {
		found = found || strings.Contains(e.Text, "zephyr rejected")
		lead = lead || strings.Contains(e.Text, "func Evaluate")
		if strings.Contains(e.Text, "zephyr rejected") {
			aligned = aligned || e.Span.StartByteColumn == 0
		}
	}
	if !found || !lead || !aligned {
		t.Fatal("decisive body, declaration lead, or readable start boundary omitted")
	}
	if res.Bundle.Omissions.Excerpts == 0 {
		t.Fatal("excerpt not marked")
	}
	assertEvidence(t, root, res.Bundle)
}

func TestM1LongLineMarksPartialQueryExcerpt(t *testing.T) {
	code := "package p\nfunc Evaluate() string {\n return \"" +
		strings.Repeat("padding", 1800) +
		"zephyr\"\n}\n"
	root, r := compileFixture(t, map[string]string{"main.go": code})
	o := baseOptions(root)
	o.Query, o.MaxBytes = "zephyr", 7000
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}

	partial, warned := false, false
	for _, evidence := range res.Bundle.Evidence {
		if strings.Contains(evidence.Text, "zephyr") {
			partial = evidence.Span.StartByteColumn != 0
		}
	}
	for _, warning := range res.Bundle.Warnings {
		warned = warned || strings.Contains(warning, "exact query excerpts use partial source-line boundaries")
	}
	if !partial || !warned {
		t.Fatalf(
			"unavoidable partial line was not marked: evidence=%+v warnings=%v",
			res.Bundle.Evidence,
			res.Bundle.Warnings,
		)
	}
	assertEvidence(t, root, res.Bundle)
}

func TestM1JSONMarkdownParityAndMerge(t *testing.T) {
	root, r := compileFixture(t, map[string]string{"doc.md": "# Gate\n\nA zephyr contract.\n\n- [ ] Require zephyr validation.\n"})
	o := baseOptions(root)
	o.Query, o.MaxBytes = "zephyr", 30000
	jsonResult, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	o.Format = "markdown"
	markdown, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(jsonResult.Bundle.Evidence, markdown.Bundle.Evidence) || !reflect.DeepEqual(jsonResult.Bundle.Omissions, markdown.Bundle.Omissions) {
		t.Fatal("format evidence or omission mismatch")
	}
	if err := markdown.Bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	assertEvidence(t, root, markdown.Bundle)
}

func TestM1SetextAndLongSections(t *testing.T) {
	doc := "Top\n===\n\n" + strings.Repeat("Padding paragraph.\n\n", 150) + "A decisive zephyr phrase.\n\nNext\n===\nLast.\n"
	root, r := compileFixture(t, map[string]string{"doc.md": doc})
	o := baseOptions(root)
	o.Query, o.MaxBytes = "zephyr", 7000
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range res.Bundle.Evidence {
		found = found || strings.Contains(ev.Text, "decisive zephyr")
	}
	if !found {
		t.Fatal("long section body unavailable")
	}
	assertEvidence(t, root, res.Bundle)
}

func TestM1DeniedTextNeverSeedsRetrieval(t *testing.T) {
	root, r := compileFixture(t, map[string]string{
		"public.md":  "# Public\n\nAn ordinary contract.\n",
		"private.md": "# Hidden\n\nforbiddenzephyrneedle\n",
	})
	o := baseOptions(root)
	o.Query, o.DenyPaths = "forbiddenzephyrneedle", []string{"private.md"}
	if _, err := Build(r, o); err == nil {
		t.Fatal("denied body seeded retrieval")
	}
	filtered, err := compiler.Compile(compiler.Options{Root: root, DenyPaths: o.DenyPaths})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range filtered.Strings {
		if strings.Contains(text, "forbiddenzephyrneedle") {
			t.Fatal("excluded source indexed")
		}
	}
	o.DenyPaths = nil
	if _, err := Build(filtered, o); err == nil {
		t.Fatal("excluded text was retrieved")
	}
}
