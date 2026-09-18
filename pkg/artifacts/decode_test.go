package artifacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

const empty = `{"version":"repoctx.artifacts/v1alpha1","namespace":"a","artifacts":[],"relationships":[]}`

func fixture(t *testing.T) *Catalog {
	t.Helper()
	b, err := os.ReadFile("testdata/claims.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Decode(b, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func wire(t *testing.T, c *Catalog) []byte {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func accepted(t *testing.T, b []byte, l Limits) *Catalog {
	t.Helper()
	c, err := Decode(b, l)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func rejected(t *testing.T, b []byte, l Limits, reason string) {
	t.Helper()
	c, err := Decode(b, l)
	if c != nil || err == nil {
		t.Fatalf("expected whole-document rejection: catalog=%v err=%v", c, err)
	}
	if !strings.Contains(err.Error(), reason) || len(err.Error()) > 256 {
		t.Fatalf("wrong or unbounded diagnostic: %s", err)
	}
	t.Logf("bytes=%d limits=%+v reason=%s", len(b), l, err)
}
func minimal() *Catalog {
	return &Catalog{Version: Version, Namespace: "a", Artifacts: []Artifact{{ID: "a:a", Kind: "component", Lifecycle: "active",
		AppliesTo: []Applicability{}, DeclaredInputs: []Input{}, Sources: []Span{{Path: "a", SHA256: strings.Repeat("0", 64),
			StartByte: 0, EndByte: 1, StartLine: 1, EndLine: 1, StartByteColumn: 0, EndByteColumn: 1}}}}, Relationships: []Relationship{}}
}
func edge() Relationship {
	return Relationship{From: "a:a", To: "a:a", Kind: "contains", Resolution: "declared", Sources: minimal().Artifacts[0].Sources}
}
func TestRoundTripClaims(t *testing.T) {
	c := fixture(t)
	got := accepted(t, wire(t, c), Limits{})
	if !reflect.DeepEqual(c, got) {
		t.Fatal("round trip lost declarations")
	}
	for _, a := range got.Artifacts {
		if a.ID != "demo:stable" || len(a.DeclaredInputs) != 5 {
			t.Fatal("duplicate declarations lost")
		}
		in := a.DeclaredInputs
		if !reflect.DeepEqual(in[0], in[1]) || *in[2].SHA256 == *in[0].SHA256 || in[3].Role != "configuration" || in[4].Required {
			t.Fatal("input conflicts lost or reordered")
		}
	}
	// Relocation and ordinary edits change provenance, never author-assigned IDs.
	got.Artifacts[0].Sources[0].Path = "moved/renamed.txt"
	got.Artifacts[0].Sources[0].SHA256 = strings.Repeat("f", 64)
	got.Artifacts[0].Sources[0].StartByte = 0
	moved := accepted(t, wire(t, got), Limits{})
	if moved.Artifacts[0].ID != c.Artifacts[0].ID {
		t.Fatal("identity changed")
	}
	for _, inputs := range [][]Input{c.Artifacts[0].DeclaredInputs[:1], {{Path: "missing.json", Role: "source", Required: false}}, {}} {
		got.Artifacts[0].DeclaredInputs = inputs
		accepted(t, wire(t, got), Limits{})
	}
	got.Artifacts[4].RunnerCheckID = nil
	got.Artifacts[4].Owner = nil
	result := accepted(t, wire(t, got), Limits{})
	if result.Artifacts[4].RunnerCheckID != nil || result.Artifacts[4].Owner != nil {
		t.Fatal("absent optionals changed")
	}
	accepted(t, []byte(empty), Limits{})
}

func TestIndependentSourceCoordinates(t *testing.T) {
	source, err := os.ReadFile("testdata/source.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, []byte{'A', 0xc3, 0xa9, '\r', '\n', 0xe7, 0x8c, 0xab, 'Z', '\n'}) {
		t.Fatal("source fixture changed")
	}
	ss := fixture(t).Artifacts[0].Sources
	digest := sha256.Sum256(source)
	expected := []Span{
		{Path: "docs/original.txt", SHA256: hex.EncodeToString(digest[:]), StartByte: 1, EndByte: 3, StartLine: 1, EndLine: 1, StartByteColumn: 1, EndByteColumn: 3},
		{Path: "docs/original.txt", SHA256: hex.EncodeToString(digest[:]), StartByte: 5, EndByte: 10, StartLine: 2, EndLine: 3, StartByteColumn: 0, EndByteColumn: 0},
	}
	if !reflect.DeepEqual(ss, expected) || string(source[1:3]) != "é" || string(source[5:10]) != "猫Z\n" {
		t.Fatal("independent coordinates disagree")
	}
	c := minimal()
	// CRLF consumes two bytes, EOF without final LF is a column on the last line.
	c.Artifacts[0].Sources = []Span{{Path: "absent", SHA256: strings.Repeat("0", 64), StartByte: 3, EndByte: 5, StartLine: 1, EndLine: 2, StartByteColumn: 3, EndByteColumn: 0},
		{Path: "absent", SHA256: strings.Repeat("0", 64), StartByte: 5, EndByte: 9, StartLine: 2, EndLine: 2, StartByteColumn: 0, EndByteColumn: 4}}
	if string(source[3:5]) != "\r\n" {
		t.Fatal("CRLF byte fixture disagrees")
	}
	noFinalLF := []byte{'A', 0xc3, 0xa9, '\r', '\n', 0xe7, 0x8c, 0xab, 'Z'}
	if len(noFinalLF) != 9 || string(noFinalLF[5:9]) != "猫Z" {
		t.Fatal("EOF fixture disagrees")
	}
	noFinalHash := sha256.Sum256(noFinalLF)
	c.Artifacts[0].Sources[0].SHA256 = hex.EncodeToString(digest[:])
	c.Artifacts[0].Sources[1].SHA256 = hex.EncodeToString(noFinalHash[:])
	accepted(t, wire(t, c), Limits{})
	// Deliberately inconsistent hash/coordinates and a mid-codepoint boundary are
	// structurally valid claims, not evidence that original bytes were verified.
	c.Artifacts[0].Sources[0].StartByte = 2
	c.Artifacts[0].Sources[0].EndByte = 900
	accepted(t, wire(t, c), Limits{})
}

func TestStrictJSON(t *testing.T) {
	base := string(wire(t, minimal()))
	cases := map[string]string{
		"duplicate":         strings.Replace(empty, `"namespace":"a"`, `"namespace":"a","namespace":"a"`, 1),
		"escaped duplicate": strings.Replace(empty, `"namespace":"a"`, `"namespace":"a","\u006eamespace":"a"`, 1),
		"unknown":           strings.Replace(empty, `"namespace":"a"`, `"namespace":"a","EXECUTE_SECRET":"x"`, 1),
		"case alias":        strings.Replace(empty, `"namespace"`, `"Namespace"`, 1),
		"version":           strings.Replace(empty, Version, "future", 1),
		"trailing":          empty + ` {}`, "garbage": empty + ` !`,
		"utf8":                 strings.Replace(empty, `"a"`, "\"\xff\"", 1),
		"high surrogate":       strings.Replace(empty, `"a"`, `"\ud800"`, 1),
		"low surrogate":        strings.Replace(empty, `"a"`, `"\udc00"`, 1),
		"surrogate pair wrong": strings.Replace(empty, `"a"`, `"\ud800\u0041"`, 1),
		"fraction":             strings.Replace(base, `"start_byte":0`, `"start_byte":0.0`, 1),
		"exponent":             strings.Replace(base, `"start_byte":0`, `"start_byte":0e0`, 1),
		"negative zero":        strings.Replace(base, `"start_byte":0`, `"start_byte":-0`, 1),
		"leading zero":         strings.Replace(base, `"start_byte":0`, `"start_byte":00`, 1),
		"range":                strings.Replace(base, `"end_byte":1`, `"end_byte":9007199254740992`, 1),
		"null":                 strings.Replace(base, `"kind":"component"`, `"kind":null`, 1),
		"nested duplicate":     strings.Replace(base, `"kind":"component"`, `"kind":"component","kind":"decision"`, 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) { rejected(t, []byte(raw), Limits{}, "") })
	}
	c := minimal()
	owner := "😀"
	c.Artifacts[0].Owner = &owner
	b := strings.Replace(string(wire(t, c)), "😀", `\ud83d\ude00`, 1)
	accepted(t, []byte(b), Limits{})
	// Exercise missing, null, and wrong types at every object level independently.
	var doc map[string]any
	if err := json.Unmarshal(wire(t, fixture(t)), &doc); err != nil {
		t.Fatal(err)
	}
	art := doc["artifacts"].([]any)[0].(map[string]any)
	objects := []map[string]any{doc, art, art["sources"].([]any)[0].(map[string]any), art["applies_to"].([]any)[0].(map[string]any), art["declared_inputs"].([]any)[0].(map[string]any), doc["relationships"].([]any)[0].(map[string]any)}
	for _, object := range objects {
		for key, value := range object {
			for _, bad := range []any{nil, map[string]any{}} {
				object[key] = bad
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				rejected(t, raw, Limits{}, "")
			}
			delete(object, key)
			if key != "sha256" || object["role"] == nil {
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				rejected(t, raw, Limits{}, "required")
			}
			object[key] = value
		}
		object["command"] = "do-not-execute"
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		rejected(t, raw, Limits{}, "unknown field")
		delete(object, "command")
	}
}

func TestPathsAndEnums(t *testing.T) {
	for _, path := range []string{"", ".", "..", "/a", "a/", "a//b", "a/./b", "a/../b", `a\b`, "C:a", "a\x00", "a\x7f", "a*", "a?", "a[b]", "a{b}", "!a"} {
		c := minimal()
		c.Artifacts[0].Sources[0].Path = path
		rejected(t, wire(t, c), Limits{}, "invalid path")
		c = minimal()
		c.Artifacts[0].DeclaredInputs = []Input{{Path: path, Role: "source"}}
		rejected(t, wire(t, c), Limits{}, "invalid path")
		c = minimal()
		c.Artifacts[0].AppliesTo = []Applicability{{Kind: "file", Path: path}}
		rejected(t, wire(t, c), Limits{}, "invalid path")
	}
	for _, path := range []string{"docs/é.txt", "docs/e\u0301.txt", "%2e%2e/file", "$HOME/file", "file name"} {
		c := minimal()
		c.Artifacts[0].Sources[0].Path = path
		got := accepted(t, wire(t, c), Limits{})
		if got.Artifacts[0].Sources[0].Path != path {
			t.Fatal("path normalized")
		}
	}
	c := minimal()
	c.Artifacts[0].AppliesTo = []Applicability{{Kind: "subtree", Path: "."}}
	accepted(t, wire(t, c), Limits{})
	changes := []func(*Catalog){
		func(c *Catalog) { c.Namespace = "A" }, func(c *Catalog) { c.Artifacts[0].ID = "other:a" },
		func(c *Catalog) { c.Artifacts[0].ID = "a:A" }, func(c *Catalog) { c.Artifacts[0].Kind = "unknown" },
		func(c *Catalog) { c.Artifacts[0].Lifecycle = "accepted" }, func(c *Catalog) { c.Artifacts[0].Sources = nil },
		func(c *Catalog) { c.Artifacts[0].Sources = []Span{} }, func(c *Catalog) { c.Artifacts[0].Sources[0].StartLine = 0 },
		func(c *Catalog) { c.Artifacts[0].Sources[0].EndByte = 0 }, func(c *Catalog) { c.Artifacts[0].Sources[0].EndByteColumn = 0 },
		func(c *Catalog) { c.Artifacts[0].Sources[0].StartLine = 2 }, func(c *Catalog) { c.Artifacts[0].Sources[0].SHA256 = "f" },
		func(c *Catalog) { s := "check"; c.Artifacts[0].RunnerCheckID = &s },
		func(c *Catalog) { c.Artifacts[0].DeclaredInputs = []Input{{Path: "a", Role: "optional"}} },
		func(c *Catalog) { c.Artifacts[0].AppliesTo = []Applicability{{Kind: "glob", Path: "a"}} },
		func(c *Catalog) { r := edge(); r.Resolution = "verified"; c.Relationships = []Relationship{r} },
		func(c *Catalog) { r := edge(); r.From = "invalid"; c.Relationships = []Relationship{r} },
		func(c *Catalog) { r := edge(); r.Kind = "executes"; c.Relationships = []Relationship{r} },
	}
	for _, change := range changes {
		c := minimal()
		change(c)
		rejected(t, wire(t, c), Limits{}, "")
	}
}

func TestInputDesignVariantsAndUnresolvedClaims(t *testing.T) {
	c := fixture(t)
	base := string(wire(t, c))
	for _, bad := range []string{"null", `"false"`, "0"} {
		rejected(t, []byte(strings.Replace(base, `"required":true`, `"required":`+bad, 1)), Limits{}, "")
	}
	rejected(t, []byte(strings.Replace(base, `"role":"source"`, `"role":"optional"`, 1)), Limits{}, "role")
	rejected(t, []byte(strings.Replace(base, `"sha256":"`+strings.Repeat("0", 64)+`"`, `"sha256":"short"`, 1)), Limits{}, "digest")
	for _, bad := range []string{"", "é", "sh -c command", "$(command)", "-check"} {
		c.Artifacts[4].RunnerCheckID = &bad
		rejected(t, wire(t, c), Limits{}, "check ID")
	}
	c = fixture(t)
	c.Relationships[0].From = "demo:stable"
	c.Relationships[0].To = "demo:next"
	c.Relationships[1].From = "demo:next"
	c.Relationships[1].To = "demo:stable"
	c.Relationships = append(c.Relationships, c.Relationships[0])
	got := accepted(t, wire(t, c), Limits{})
	if !reflect.DeepEqual(got, c) {
		t.Fatal("cycle or duplicate relationship was resolved")
	}
}
