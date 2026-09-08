package goast

import (
	"github.com/tdeshazo/repoctx/pkg/ir"
	"testing"
)

func TestParse(t *testing.T) {
	src := []byte(`package demo
import "fmt"
type User struct{ Name string }
func Hello(u User) string { return fmt.Sprintf("hi %s", u.Name) }
`)
	st := ir.NewStrings()
	r, err := Parse(src, st)
	if err != nil {
		t.Fatal(err)
	}
	if r.Unit != "demo" {
		t.Fatalf("unit=%q", r.Unit)
	}
	if len(r.Nodes) < 10 || len(r.Roots) != 1 {
		t.Fatalf("nodes=%d roots=%v", len(r.Nodes), r.Roots)
	}
	var haveType, haveFunc, haveImport, haveCall bool
	for _, s := range r.Symbols {
		if s.Name == "User" && s.Kind == ir.SymType {
			haveType = true
		}
		if s.Name == "Hello" && s.Kind == ir.SymFunction {
			haveFunc = true
		}
	}
	for _, e := range r.Edges {
		if e.Kind == ir.EdgeImports && e.Text == "fmt" {
			haveImport = true
		}
		if e.Kind == ir.EdgeCalls && e.Text == "fmt.Sprintf" {
			haveCall = true
		}
	}
	if !haveType || !haveFunc || !haveImport || !haveCall {
		t.Fatalf("type=%v func=%v import=%v call=%v", haveType, haveFunc, haveImport, haveCall)
	}
}
