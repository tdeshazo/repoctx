package main

import (
	"os"
	"path/filepath"
)

// Atomically replace files after the complete bundle passes budget checks. A
// failure before rename never replaces a previously valid output. Explicit
// output paths are controlled by the caller, not inferred from repository text.
func writePayload(path string, b []byte) error {
	if path == "" || path == "-" {
		_, e := os.Stdout.Write(b)
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".repoctx-context-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(temp, path)
}
