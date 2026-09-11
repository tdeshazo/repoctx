package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// InputManifest declares all filesystem bytes and negative dependencies used
// by compilation. It is content identity, not authentication or provenance.
type InputManifest struct {
	Profile CompilationProfile `json:"profile"`
	Sources []Input            `json:"sources"`
	GoMod   Input              `json:"go_mod"`
}

// Input contains metadata only; unavailable inputs reveal no observed content.
type Input struct {
	Path   string `json:"path"`
	State  string `json:"state"` // present, absent, or unavailable (policy denied)
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
}

// CompilationProfile records implementation identity and the effective caller
// policy. IgnoreDirs is a sorted list of basenames, not repository ignore rules.
type CompilationProfile struct {
	Compiler     string   `json:"compiler"`
	Frontends    []string `json:"frontends"`
	Build        string   `json:"build"`
	Allow        []string `json:"allow"`
	Deny         []string `json:"deny"`
	IgnoreDirs   []string `json:"ignore_dirs"`
	MaxFileBytes int64    `json:"max_file_bytes"`
	MaxReadBytes int64    `json:"max_read_bytes"`
	MaxEntries   int      `json:"max_entries"`
}

// ContentID hashes canonical Go JSON, without host paths or timestamps.
func ContentID(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// SourceID binds inventory, content, and optional dependency states independently
// of compiler implementation and selection limits.
func (m *InputManifest) SourceID() (string, error) {
	return ContentID(struct {
		Sources []Input
		GoMod   Input
	}{Sources: m.Sources, GoMod: m.GoMod})
}

// ProfileID distinguishes compilation implementations, scopes, and limits.
func (m *InputManifest) ProfileID() (string, error) { return ContentID(m.Profile) }

// CanonicalPrefixes normalizes a caller policy without changing its meaning.
func CanonicalPrefixes(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, strings.TrimSuffix(p, "/"))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func validInputPath(p string) bool {
	return fs.ValidPath(p) && p != "." && !strings.ContainsAny(p, "\\\x00:*")
}

func (r *Repository) validateInputs() error {
	m := r.Inputs
	if m == nil {
		return fmt.Errorf("missing compilation-input manifest")
	}
	p := m.Profile
	if p.Compiler == "" || len(p.Frontends) == 0 || p.Build == "" {
		return fmt.Errorf("missing compiler/frontend/build profile")
	}
	if p.MaxFileBytes < 1 || p.MaxFileBytes > 16<<20 || p.MaxReadBytes < 1 || p.MaxReadBytes > 256<<20 {
		return fmt.Errorf("invalid compilation read limits")
	}
	if p.MaxEntries < 1 || p.MaxEntries > 100000 {
		return fmt.Errorf("invalid inventory limit")
	}
	for _, paths := range [][]string{p.Allow, p.Deny, p.IgnoreDirs} {
		if paths == nil || !slices.Equal(paths, CanonicalPrefixes(paths)) {
			return fmt.Errorf("noncanonical compilation policy")
		}
		for _, path := range paths {
			if !validInputPath(path) {
				return fmt.Errorf("unsafe compilation policy")
			}
		}
	}
	for _, dir := range p.IgnoreDirs {
		if strings.Contains(dir, "/") {
			return fmt.Errorf("ignore directory must be a basename")
		}
	}
	if m.Sources == nil || len(m.Sources) != len(r.Files) || len(m.Sources) > p.MaxEntries {
		return fmt.Errorf("manifest source inventory disagrees with index")
	}
	var total int64
	for i, in := range m.Sources {
		f := r.Files[i]
		if in.Path != r.String(f.Path) || in.SHA256 != f.Hash || in.State != "present" {
			return fmt.Errorf("manifest source disagrees with indexed file")
		}
		if i > 0 && in.Path <= m.Sources[i-1].Path {
			return fmt.Errorf("unsorted source inventory")
		}
		if !p.Permits(in.Path) {
			return fmt.Errorf("manifest source violates scope")
		}
		parts := strings.Split(in.Path, "/")
		for _, dir := range parts[:len(parts)-1] {
			if slices.Contains(p.IgnoreDirs, dir) {
				return fmt.Errorf("manifest source violates directory exclusions")
			}
		}
		if in.Bytes < 0 || in.Bytes > p.MaxFileBytes {
			return fmt.Errorf("invalid input size")
		}
		total += in.Bytes
	}
	in := m.GoMod
	if in.Path != "go.mod" {
		return fmt.Errorf("invalid auxiliary dependency")
	}
	if !p.Permits(in.Path) {
		if in.State != "unavailable" {
			return fmt.Errorf("denied auxiliary input is not unavailable")
		}
	} else if in.State != "present" && in.State != "absent" {
		return fmt.Errorf("invalid auxiliary input state")
	}
	if in.State == "present" {
		hash, err := hex.DecodeString(in.SHA256)
		if err != nil || len(hash) != 32 || in.Bytes < 0 || in.Bytes > min(p.MaxFileBytes, 1<<20) {
			return fmt.Errorf("invalid auxiliary input digest or size")
		}
	} else if in.SHA256 != "" || in.Bytes != 0 {
		return fmt.Errorf("unread dependency has content metadata")
	}
	if total+in.Bytes > p.MaxReadBytes {
		return fmt.Errorf("manifest exceeds read limit")
	}
	return nil
}

// Permits applies a validated caller policy; deny prefixes always win.
func (p CompilationProfile) Permits(path string) bool {
	match := func(prefix string) bool { return path == prefix || strings.HasPrefix(path, prefix+"/") }
	for _, prefix := range p.Deny {
		if match(prefix) {
			return false
		}
	}
	if len(p.Allow) == 0 {
		return true
	}
	for _, prefix := range p.Allow {
		if match(prefix) {
			return true
		}
	}
	return false
}
