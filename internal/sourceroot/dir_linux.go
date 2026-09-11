//go:build linux

package sourceroot

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
)

// ReadDir enumerates at most limit entries without following directory symlinks.
// Enumeration, like Read, is not an atomic snapshot of concurrent writers.
func (r *Root) ReadDir(path string, limit int) ([]fs.DirEntry, error) {
	if limit < 1 || !fs.ValidPath(path) || strings.ContainsAny(path, "\\\x00:") {
		return nil, fmt.Errorf("unsafe directory path or limit")
	}
	fd, err := syscall.Openat(r.fd, ".", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	if path != "." {
		for _, part := range strings.Split(path, "/") {
			next, e := syscall.Openat(fd, part,
				syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
			syscall.Close(fd)
			if e != nil {
				return nil, e
			}
			fd = next
		}
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	return f.ReadDir(limit)
}
