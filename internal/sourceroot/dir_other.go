//go:build !linux

package sourceroot

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReadDir requires an immutable access-controlled tree on non-Linux platforms,
// for the same check/open race reason as Read.
func (r *Root) ReadDir(path string, limit int) ([]fs.DirEntry, error) {
	if limit < 1 || !fs.ValidPath(path) || strings.ContainsAny(path, "\\\x00:") {
		return nil, fmt.Errorf("unsafe directory path or limit")
	}
	p := r.path
	if path != "." {
		for _, part := range strings.Split(path, "/") {
			p = filepath.Join(p, part)
			info, err := os.Lstat(p)
			if err != nil {
				return nil, err
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("non-directory or symlink denied")
			}
		}
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(limit)
}
