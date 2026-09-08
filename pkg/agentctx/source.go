package agentctx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

type source struct {
	data       []byte
	lines      []int
	path, hash string
}

func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func allowed(path string, o Options) bool {
	match := func(p string) bool {
		p = strings.TrimSuffix(p, "/")
		return path == p || strings.HasPrefix(path, p+"/")
	}
	for _, p := range o.DenyPaths {
		if match(p) {
			return false
		}
	}
	if len(o.AllowPaths) == 0 {
		return true
	}
	for _, p := range o.AllowPaths {
		if match(p) {
			return true
		}
	}
	return false
}
func validatePrefixes(o Options) error {
	for _, ps := range [][]string{o.AllowPaths, o.DenyPaths} {
		for _, p := range ps {
			q := strings.TrimSuffix(p, "/")
			if !fs.ValidPath(q) || q == "." || strings.ContainsAny(q, "\\\x00:*") {
				return fmt.Errorf("invalid path prefix %q; use a relative file/directory, not a glob", p)
			}
		}
	}
	return nil
}
func loadSources(r *ir.Repository, o Options) (map[int]*source, error) {
	root, e := sourceroot.Open(o.Root)
	if e != nil {
		return nil, e
	}
	defer root.Close()
	sources := map[int]*source{}
	var total int64
	for i, f := range r.Files {
		p := r.String(f.Path)
		if !allowed(p, o) {
			continue
		}
		b, e := root.Read(p, o.MaxSourceBytes)
		if e != nil {
			return nil, fmt.Errorf("read indexed file %q: %w", p, e)
		}
		total += int64(len(b))
		if total > o.MaxReadBytes {
			return nil, fmt.Errorf("indexed source verification exceeds total read limit")
		}
		if hashBytes(b) != f.Hash {
			return nil, fmt.Errorf("stale index: %q changed; recompile before serving context", p)
		}
		if !utf8.Valid(b) {
			return nil, fmt.Errorf("source %q is not UTF-8", p)
		}
		starts := []int{0}
		for j, c := range b {
			if c == '\n' {
				starts = append(starts, j+1)
			}
		}
		sources[i] = &source{data: b, lines: starts, path: p, hash: f.Hash}
	}
	return sources, nil
}
func (s *source) position(line, col int) (int, error) {
	if line < 1 || line > len(s.lines) || col < 0 {
		return 0, fmt.Errorf("out-of-range source position")
	}
	start := s.lines[line-1]
	limit := len(s.data)
	if line < len(s.lines) {
		limit = s.lines[line] - 1
	}
	if col > limit-start {
		return 0, fmt.Errorf("source byte column exceeds line")
	}
	off := start + col
	if off < len(s.data) && !utf8.RuneStart(s.data[off]) {
		return 0, fmt.Errorf("source position splits UTF-8 rune")
	}
	return off, nil
}
func (s *source) offsets(p ir.Span) (int, int, error) {
	a, e := s.position(p.SL, p.SC)
	if e != nil {
		return 0, 0, e
	}
	b, e := s.position(p.EL, p.EC)
	if e != nil {
		return 0, 0, e
	}
	if b <= a {
		return 0, 0, fmt.Errorf("empty or reversed evidence span")
	}
	return a, b, nil
}
func (s *source) point(off int) (int, int) {
	line := sort.Search(len(s.lines), func(i int) bool { return s.lines[i] > off })
	if line == 0 {
		line = 1
	}
	return line, off - s.lines[line-1]
}
func (s *source) span(a, b int) Span {
	sl, sc := s.point(a)
	el, ec := s.point(b)
	return Span{sl, sc, el, ec}
}

// Include adjacent Go line-docs and the `type`/`var`/`const` token omitted by
// TypeSpec/ValueSpec spans. Never rewrite bytes; Definition remains the AST span.
func (s *source) declarationStart(a int, lang ir.Language) int {
	line, col := s.point(a)
	start := s.lines[line-1]
	prefix := strings.TrimSpace(string(s.data[start:a]))
	if prefix == "" || lang == ir.LangGo && (prefix == "type" || prefix == "var" || prefix == "const") {
		a = start
		col = 0
	}
	if lang == ir.LangGo && col == 0 {
		for line > 1 {
			prev := s.lines[line-2]
			text := strings.TrimSpace(string(s.data[prev:a]))
			if !strings.HasPrefix(text, "//") {
				break
			}
			a = prev
			line--
		}
	}
	return a
}
func (s *source) excerpt(a, b, byteLimit int) (int, int) {
	limit := a + byteLimit
	if b <= limit {
		return a, b
	}
	if limit > len(s.data) {
		limit = len(s.data)
	}
	if p := bytes.LastIndexByte(s.data[a:limit], '\n'); p >= 0 {
		limit = a + p + 1
	} else {
		for limit > a && !utf8.RuneStart(s.data[limit]) {
			limit--
		}
	}
	return a, limit
}
func evidenceID(s *source, a, b int) string {
	return "e:" + hashBytes([]byte(fmt.Sprintf("%s:%s:%d:%d", s.path, s.hash, a, b)))[:24]
}

// addEvidence coalesces overlapping source spans and redirects existing symbols
// to the new shared evidence block. Code bytes occur only once in the payload.
func addEvidence(b *Bundle, s *source, a, z int, role string) string {
	old := map[string]bool{}
	changed := true
	for changed {
		changed = false
		for _, e := range b.Evidence {
			if e.File == s.path && !old[e.ID] && a < e.EndByte && e.StartByte < z {
				old[e.ID] = true
				if e.StartByte < a {
					a = e.StartByte
				}
				if e.EndByte > z {
					z = e.EndByte
				}
				if e.Role == "source" {
					role = "source"
				}
				changed = true
			}
		}
	}
	id := evidenceID(s, a, z)
	keep := make([]Evidence, 0, len(b.Evidence)+1)
	for _, e := range b.Evidence {
		if !old[e.ID] {
			keep = append(keep, e)
		}
	}
	keep = append(keep, Evidence{ID: id, File: s.path, SHA256: s.hash, Span: s.span(a, z), StartByte: a, EndByte: z, Role: role, Text: string(s.data[a:z])})
	sort.Slice(keep, func(i, j int) bool {
		if keep[i].File != keep[j].File {
			return keep[i].File < keep[j].File
		}
		return keep[i].StartByte < keep[j].StartByte
	})
	b.Evidence = keep
	for i := range b.Symbols {
		if old[b.Symbols[i].Evidence] {
			b.Symbols[i].Evidence = id
		}
	}
	return id
}
