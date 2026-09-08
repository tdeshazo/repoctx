//go:build linux

// Package sourceroot opens evidence within a caller-selected repository root.
package sourceroot

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
)

type Root struct{ fd int }

func Open(path string) (*Root, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	return &Root{fd: fd}, nil
}
func (r *Root) Close() error { return syscall.Close(r.fd) }

// Read rejects all symlinks and non-regular files. Each path component is opened
// relative to an existing directory descriptor with O_NOFOLLOW, avoiding a
// check-then-open symlink race. Trusted callers must still isolate mounts and
// concurrent repository writers; a set of reads is not an atomic FS snapshot.
func (r *Root) Read(path string, maxBytes int64) ([]byte, error) {
	if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00:") {
		return nil, fmt.Errorf("unsafe path")
	}
	parts := strings.Split(path, "/")
	dir := r.fd
	owned := false
	defer func() {
		if owned {
			syscall.Close(dir)
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		fd, e := syscall.Openat(dir, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if e != nil {
			return nil, e
		}
		if owned {
			syscall.Close(dir)
		}
		dir = fd
		owned = true
	}
	fd, e := syscall.Openat(dir, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("evidence is not a regular file")
	}
	if maxBytes < 1 || info.Size() > maxBytes {
		return nil, fmt.Errorf("source exceeds read limit")
	}
	b, e := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > maxBytes {
		return nil, fmt.Errorf("source exceeds read limit")
	}
	return b, nil
}
