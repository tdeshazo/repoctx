package ir

import "testing"

func TestBuildCSR(t *testing.T) {
	arcs := []GraphArc{
		{From: 0, To: 1, Kind: EdgeCalls, Weight: 1},
		{From: 0, To: 1, Kind: EdgeCalls, Weight: 2},
		{From: 1, To: 2, Kind: EdgeDefines, Weight: 1},
	}
	out, in := BuildCSR(3, arcs)
	if got, want := out.Offsets, []uint32{0, 1, 2, 2}; !u32eq(got, want) {
		t.Fatalf("out offsets=%v want=%v", got, want)
	}
	if len(out.Targets) != 2 || out.Targets[0] != 1 || out.Kinds[0] != EdgeCalls || out.Weights[0] != 3 {
		t.Fatalf("collapsed out=%+v", out)
	}
	if got, want := in.Offsets, []uint32{0, 0, 1, 2}; !u32eq(got, want) {
		t.Fatalf("in offsets=%v want=%v", got, want)
	}
	if in.Targets[0] != 0 || in.Weights[0] != 3 {
		t.Fatalf("reverse=%+v", in)
	}
}

func u32eq(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
