package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritePayloadAtomicReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "context.json")
	if e := os.WriteFile(path, []byte("old"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := writePayload(path, []byte("{\"ok\":true}\n")); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if string(b) != "{\"ok\":true}\n" {
		t.Fatal("partial replacement")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary output leaked")
	}
}
func TestWritePayloadFailureDoesNotLeaveTemp(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "target")
	os.Mkdir(path, 0755)
	os.WriteFile(filepath.Join(path, "keep"), []byte("safe"), 0600)
	if e := writePayload(path, []byte("cannot replace nonempty dir")); e == nil {
		t.Fatal("expected write failure")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
}
