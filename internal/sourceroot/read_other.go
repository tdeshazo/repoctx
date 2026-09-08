//go:build !linux

package sourceroot

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Portable fallback: use only with an immutable, access-controlled worktree.
// Unlike the Linux descriptor-relative implementation this cannot eliminate
// concurrent directory replacement between validation and open on Go 1.23.
type Root struct{ path string }

func Open(path string) (*Root, error) {
	p, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	p, e = filepath.EvalSymlinks(p)
	if e != nil {
		return nil, e
	}
	return &Root{p}, nil
}
func (r *Root) Close() error { return nil }
func (r *Root) Read(path string, maxBytes int64) ([]byte, error) {
	if !fs.ValidPath(path) || path == "." || strings.ContainsAny(path, "\\\x00:") {
		return nil, fmt.Errorf("unsafe path")
	}
	p := r.path
	for _, s := range strings.Split(path, "/") {
		p = filepath.Join(p, s)
		info, e := os.Lstat(p)
		if e != nil {
			return nil, e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink source denied")
		}
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || maxBytes < 1 || info.Size() > maxBytes {
		return nil, fmt.Errorf("invalid source type/size")
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
