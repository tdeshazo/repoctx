//go:build linux

package discovery

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestFIFOIsNotOpened(t *testing.T) {
	o := DefaultOptions()
	o.Root = t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(o.Root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	o.Operation, o.Query = "search", "needle"
	r := run(t, o)
	if !hasReason(r, "symlink_or_special_file") || len(r.Response.Results) != 0 {
		t.Fatal("special file handling")
	}
}
