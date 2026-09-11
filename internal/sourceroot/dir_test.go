package sourceroot

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestConfinedDirectoryEnumeration(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "file"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for i := 0; i < 2; i++ {
		entries, err := r.ReadDir("nested", 10)
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "file" {
			t.Fatal(entries)
		}
	}
	for _, p := range []string{"link", "../", "/tmp", "nested/file"} {
		if _, err := r.ReadDir(p, 10); err == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	entries, err := r.ReadDir(".", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatal("limit not applied")
	}
}
