package ir

import "testing"

func TestStringsIntern(t *testing.T) {
	s := NewStrings()
	if got := s.Intern(""); got != 0 {
		t.Fatalf("empty=%d", got)
	}
	a := s.Intern("alpha")
	b := s.Intern("alpha")
	if a != b || a == 0 {
		t.Fatalf("intern not stable: %d %d", a, b)
	}
	if got := s.Values(); len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("values=%v", got)
	}
}
