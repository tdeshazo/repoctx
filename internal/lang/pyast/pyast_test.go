package pyast

import (
	"github.com/tdeshazo/repoctx/pkg/ir"
	"testing"
)

func TestParse(t *testing.T) {
	src := []byte(`import json
class User:
    def greet(self, name: str) -> str:
        return json.dumps({"hello": name})
`)
	st := ir.NewStrings()
	// The legacy interpreter argument is ignored; this also proves that Python
	// indexing does not depend on an executable in PATH.
	r, err := Parse("definitely-not-a-python-executable", src, st)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nodes) < 8 || len(r.Roots) != 1 {
		t.Fatalf("nodes=%d roots=%v", len(r.Nodes), r.Roots)
	}
	var haveType, haveMethod, haveImport, haveCall bool
	for _, s := range r.Symbols {
		if s.Name == "User" && s.Kind == ir.SymType {
			haveType = true
		}
		if s.Name == "greet" && s.Kind == ir.SymMethod {
			haveMethod = true
		}
	}
	for _, e := range r.Edges {
		if e.Kind == ir.EdgeImports && e.Text == "json" {
			haveImport = true
		}
		if e.Kind == ir.EdgeCalls && e.Text == "json.dumps" {
			haveCall = true
		}
	}
	if !haveType || !haveMethod || !haveImport || !haveCall {
		t.Fatalf("type=%v method=%v import=%v call=%v", haveType, haveMethod, haveImport, haveCall)
	}
}

func TestLimitedBufferBoundsHelperOutput(t *testing.T) {
	b := &limitedBuffer{limit: 4}
	if n, err := b.Write([]byte("abcd")); err != nil || n != 4 {
		t.Fatalf("initial write: n=%d err=%v", n, err)
	}
	if n, err := b.Write([]byte("ef")); n != 0 || err == nil {
		t.Fatalf("write after limit: n=%d err=%v", n, err)
	}
	if got := b.String(); got != "abcd" {
		t.Fatalf("buffer changed after limit: %q", got)
	}
}
