package artifacts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestGenerateResolvesExactSourceEvidence(t *testing.T) {
	root := t.TempDir()
	content := []byte("zero\r\nα start middle end\r\n")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	wire, err := Generate(root, authoringFixture("α start", "end"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(wire, []byte("\n")) {
		t.Fatal("generated catalog lacks final newline")
	}
	catalog, err := Decode(wire, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	span := catalog.Artifacts[0].Sources[0]
	if span.StartByte != 6 || span.EndByte != 25 || span.StartLine != 2 || span.EndLine != 2 ||
		span.StartByteColumn != 0 || span.EndByteColumn != 19 {
		t.Fatalf("unexpected generated span: %+v", span)
	}
	for _, line := range bytes.Split(wire, []byte("\n")) {
		if len(line) > 2048 {
			t.Fatalf("generated catalog line is too large for bounded retrieval: %d", len(line))
		}
	}
}

func TestGenerateRejectsInvalidOrAmbiguousAuthoring(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("start end start\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, wire := range map[string][]byte{
		"duplicate member": bytes.Replace(authoringFixture("start", "end"), []byte(`"namespace":"test",`), []byte(`"namespace":"test","namespace":"test",`), 1),
		"unknown member":   bytes.Replace(authoringFixture("start", "end"), []byte(`"namespace":"test",`), []byte(`"namespace":"test","extra":true,`), 1),
		"ambiguous anchor": authoringFixture("start", "end"),
		"unused anchor": bytes.Replace(authoringFixture("missing", "end"), []byte(`"source_anchors":[`),
			[]byte(`"source_anchors":[{"id":"unused","path":"source.txt","start":"missing","end":"end"},`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Generate(root, wire, Limits{})
			if err == nil {
				t.Fatal("expected authoring failure")
			}
			if strings.Contains(err.Error(), "start end start") {
				t.Fatal("diagnostic copied repository source")
			}
		})
	}
}

func TestEntitiesUsesCanonicalCompilation(t *testing.T) {
	root := t.TempDir()
	content := []byte("start middle end\n")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	model, err := Entities(root, authoringFixture("start", "end"), func(path string) bool {
		return path == "source.txt"
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Version != ir.EntityVersion || len(model.Entities) != 1 || len(model.Relationships) != 1 {
		t.Fatalf("unexpected entity adapter result: %+v", model)
	}
	entity := model.Entities[0]
	if entity.ID != "test:component.sample" || entity.Sensitivity != "internal" ||
		entity.Owners == nil || entity.Supersedes == nil || entity.Freshness.Inputs == nil ||
		len(entity.Declaration.Sources) != 1 || entity.Declaration.Sources[0].EndByte != int64(len("start middle end")) {
		t.Fatalf("artifact declaration was not preserved: %+v", entity)
	}
	if relationship := model.Relationships[0]; relationship.From != entity.ID || relationship.To != entity.ID ||
		relationship.Kind != "references" || len(relationship.Declaration.Sources) != 1 {
		t.Fatalf("artifact relationship was not preserved: %+v", relationship)
	}

	if denied, err := Entities(root, authoringFixture("start", "end"), func(string) bool { return false }); err == nil || denied != nil || !strings.Contains(err.Error(), "outside caller scope") {
		t.Fatalf("denied anchor was read: model=%+v err=%v", denied, err)
	}
}

func authoringFixture(start, end string) []byte {
	doc := `{
  "version":"repoctx.artifact-authoring/v1alpha2",
  "namespace":"test",
  "source_anchors":[{"id":"sample","path":"source.txt","start":START,"end":END}],
  "artifacts":[{
    "id":"test:component.sample",
    "kind":"component",
    "applies_to":[{"kind":"file","path":"source.txt"}],
    "lifecycle":"active",
    "sensitivity":"internal",
    "sources":["sample"],
    "declared_inputs":[]
  }],
  "relationships":[{
    "from":"test:component.sample",
    "to":"test:component.sample",
    "kind":"references",
    "resolution":"declared",
    "sources":["sample"]
  }]
}`
	startJSON, _ := json.Marshal(start)
	endJSON, _ := json.Marshal(end)
	doc = strings.Replace(doc, "START", string(startJSON), 1)
	doc = strings.Replace(doc, "END", string(endJSON), 1)
	return []byte(doc)
}
