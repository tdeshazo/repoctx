package compiler

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

const compilerIdentity = "repoctx.compiler/m2.1"
const buildProfile = "syntax-only/all-supported-files/no-build-tags/no-ignore-files/no-external-providers"

func frontends() []string {
	ids := []string{"go/parser:" + runtime.Version(), "tree-sitter-lowering/m1.1"}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if !strings.Contains(dep.Path, "tree-sitter") {
				continue
			}
			original := dep.Path
			if dep.Replace != nil {
				dep = dep.Replace
			}
			// Local replacements have no reproducible content identity. Keep the
			// marker, never a host-specific replacement path; reject them below.
			if dep.Version == "" {
				ids = append(ids, "unidentified-local-frontend")
				continue
			}
			identity := dep.Path + "@" + dep.Version + ":" + dep.Sum
			if original != dep.Path {
				identity = original + "=>" + identity
			}
			ids = append(ids, identity)
		}
	}
	slices.Sort(ids)
	return ids
}

func compilationProfile(o Options) ir.CompilationProfile {
	ignored := []string{}
	for name, enabled := range o.IgnoreDirs {
		if enabled {
			ignored = append(ignored, name)
		}
	}
	slices.Sort(ignored)
	return ir.CompilationProfile{
		Compiler: compilerIdentity, Frontends: frontends(), Build: buildProfile,
		Allow: ir.CanonicalPrefixes(o.AllowPaths), Deny: ir.CanonicalPrefixes(o.DenyPaths), IgnoreDirs: ignored,
		MaxFileBytes: o.MaxBytes, MaxReadBytes: o.MaxReadBytes, MaxEntries: o.MaxEntries,
	}
}

func compatibleProfile(p ir.CompilationProfile) bool {
	return p.Compiler == compilerIdentity && p.Build == buildProfile &&
		slices.Equal(p.Frontends, frontends()) && !slices.Contains(p.Frontends, "unidentified-local-frontend")
}

// inventory is bounded and descriptor-relative on Linux. It enumerates parent
// entry names to locate allowed descendants, but never descends denied trees.
// Symlinks and special files are outside the source discovery boundary.
func inventory(root *sourceroot.Root, p ir.CompilationProfile) ([]string, error) {
	paths := []string{}
	pending := []string{"."}
	remaining := p.MaxEntries
	for len(pending) > 0 {
		dir := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		entries, err := root.ReadDir(dir, remaining+1)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("incomplete source inventory: %w", err)
		}
		remaining -= len(entries)
		if remaining < 0 {
			return nil, fmt.Errorf("source inventory exceeds entry limit")
		}
		for _, entry := range entries {
			path := entry.Name()
			if dir != "." {
				path = dir + "/" + path
			}
			if entry.IsDir() {
				if slices.Contains(p.IgnoreDirs, entry.Name()) || !mayDescend(path, p) {
					continue
				}
				pending = append(pending, path)
				continue
			}
			if entry.Type().IsRegular() && p.Permits(path) && sourceLanguage(path) != ir.LangUnknown {
				paths = append(paths, path)
			}
		}
	}
	slices.Sort(paths)
	return paths, nil
}

func mayDescend(path string, p ir.CompilationProfile) bool {
	for _, deny := range p.Deny {
		if path == deny || strings.HasPrefix(path, deny+"/") {
			return false
		}
	}
	if len(p.Allow) == 0 {
		return true
	}
	for _, allow := range p.Allow {
		if path == allow || strings.HasPrefix(path, allow+"/") || strings.HasPrefix(allow, path+"/") {
			return true
		}
	}
	return false
}

func readInput(root *sourceroot.Root, path string, limit int64, remaining *int64) (ir.Input, []byte, error) {
	if *remaining < 0 {
		return ir.Input{}, nil, fmt.Errorf("total compilation/read limit exhausted")
	}
	b, err := root.Read(path, min(limit, *remaining))
	if err != nil {
		return ir.Input{}, nil, err
	}
	*remaining -= int64(len(b))
	return ir.Input{Path: path, State: "present", SHA256: fullHash(b), Bytes: int64(len(b))}, b, nil
}

func captureInputs(root *sourceroot.Root, p ir.CompilationProfile, remaining *int64) (*ir.InputManifest, map[string][]byte, error) {
	if !compatibleProfile(p) {
		return nil, nil, fmt.Errorf("incompatible or unidentified compiler/frontend profile; recompile")
	}
	paths, err := inventory(root, p)
	if err != nil {
		return nil, nil, err
	}
	m := &ir.InputManifest{Profile: p, Sources: []ir.Input{}, GoMod: ir.Input{Path: "go.mod", State: "unavailable"}}
	contents := map[string][]byte{}
	for _, path := range paths {
		in, b, err := readInput(root, path, p.MaxFileBytes, remaining)
		if err != nil {
			return nil, nil, fmt.Errorf("incomplete compilation input %q: %w", path, err)
		}
		m.Sources = append(m.Sources, in)
		contents[path] = b
	}
	if p.Permits("go.mod") {
		in, b, err := readInput(root, "go.mod", min(p.MaxFileBytes, 1<<20), remaining)
		if errors.Is(err, fs.ErrNotExist) {
			in = ir.Input{Path: "go.mod", State: "absent"}
		} else if err != nil {
			return nil, nil, fmt.Errorf("auxiliary compilation input: %w", err)
		}
		m.GoMod = in
		contents["go.mod"] = b
	}
	finalPaths, err := inventory(root, p)
	if err != nil {
		return nil, nil, err
	}
	if !slices.Equal(paths, finalPaths) {
		return nil, nil, fmt.Errorf("source inventory changed during input reads")
	}
	return m, contents, nil
}

func verifyInputs(root *sourceroot.Root, expected *ir.InputManifest, remaining *int64) error {
	actual, _, err := captureInputs(root, expected.Profile, remaining)
	if err != nil {
		return fmt.Errorf("stale or incomplete compilation inputs: %w", err)
	}
	a, _ := actual.SourceID()
	b, _ := expected.SourceID()
	if a != b {
		return fmt.Errorf("stale compilation inputs: inventory or content changed; recompile")
	}
	return nil
}

// LoadInputs verifies the declared generation and returns the exact indexed
// source bytes. Immutable mode is a caller assertion, not repository-controlled
// configuration: it skips inventory/auxiliary revalidation but still hashes all
// source bytes. Callers must pin and authenticate the index and isolate writers.
func LoadInputs(r *ir.Repository, rootPath string, immutable bool, maxFile, maxRead int64) (map[string][]byte, error) {
	if maxFile < 1 || maxRead < 1 {
		return nil, fmt.Errorf("source read limits must be positive")
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if r.Inputs == nil || !compatibleProfile(r.Inputs.Profile) {
		return nil, fmt.Errorf("incompatible compilation profile; recompile")
	}
	root, err := sourceroot.Open(rootPath)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	remaining := min(maxRead, r.Inputs.Profile.MaxReadBytes)
	if immutable {
		contents := map[string][]byte{}
		for _, expected := range r.Inputs.Sources {
			actual, b, err := readInput(root, expected.Path, min(maxFile, r.Inputs.Profile.MaxFileBytes), &remaining)
			if err != nil {
				return nil, err
			}
			if actual != expected {
				return nil, fmt.Errorf("stale immutable snapshot source; recompile")
			}
			contents[actual.Path] = b
		}
		return contents, nil
	}
	// A caller may tighten byte limits, but never expand the manifest's scope.
	p := r.Inputs.Profile
	p.MaxFileBytes = min(p.MaxFileBytes, maxFile)
	actual, contents, err := captureInputs(root, p, &remaining)
	if err != nil {
		return nil, fmt.Errorf("stale or incomplete compilation inputs: %w", err)
	}
	a, _ := actual.SourceID()
	b, _ := r.Inputs.SourceID()
	if a != b {
		return nil, fmt.Errorf("stale compilation inputs: inventory or content changed; recompile")
	}
	if err := verifyInputs(root, actual, &remaining); err != nil {
		return nil, err
	}
	return contents, nil
}
