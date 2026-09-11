package compiler

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/internal/sourceroot"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

func inputFixture(t *testing.T, options Options) (string, *ir.Repository) {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "main.go"), "package main\nfunc Answer() {}\n")
	options.Root = root
	r, err := Compile(options)
	if err != nil {
		t.Fatal(err)
	}
	return root, r
}

func TestM2InventoryAndAuxiliaryDrift(t *testing.T) {
	changes := map[string]func(*testing.T, string){
		"addition": func(t *testing.T, root string) { mustWrite(t, filepath.Join(root, "untracked.py"), "value = 1\n") },
		"deletion": func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
				t.Fatal(err)
			}
		},
		"rename": func(t *testing.T, root string) {
			if err := os.Rename(filepath.Join(root, "main.go"), filepath.Join(root, "renamed.go")); err != nil {
				t.Fatal(err)
			}
		},
		"modification": func(t *testing.T, root string) {
			mustWrite(t, filepath.Join(root, "main.go"), "package main\nfunc Changed() {}\n")
		},
		"negative dependency": func(t *testing.T, root string) { mustWrite(t, filepath.Join(root, "go.mod"), "module new.example\n") },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			root, r := inputFixture(t, Options{})
			if r.Inputs.GoMod.State != "absent" {
				t.Fatal("optional input absence not recorded")
			}
			change(t, root)
			if _, err := LoadInputs(r, root, false, 2<<20, 256<<20); err == nil {
				t.Fatal("changed inputs accepted")
			}
			updated, err := Compile(Options{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			oldID, _ := r.Inputs.SourceID()
			newID, _ := updated.Inputs.SourceID()
			if oldID == newID {
				t.Fatal("source identity unchanged")
			}
		})
	}
	for _, change := range []string{"modify", "delete"} {
		t.Run("present go.mod "+change, func(t *testing.T) {
			root, _ := inputFixture(t, Options{})
			mustWrite(t, filepath.Join(root, "go.mod"), "module first.example\n")
			r, err := Compile(Options{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			if change == "modify" {
				mustWrite(t, filepath.Join(root, "go.mod"), "module second.example\n")
			} else if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadInputs(r, root, false, 2<<20, 256<<20); err == nil {
				t.Fatal("go.mod drift accepted")
			}
		})
	}
}

func TestM2DiscoveryBoundaryAndScope(t *testing.T) {
	options := Options{AllowPaths: []string{"main.go"}, DenyPaths: []string{"private"}}
	root, r := inputFixture(t, options)
	for _, path := range []string{"other.go", "private/secret.go", "vendor/ignored.go", "unsupported.txt"} {
		mustWrite(t, filepath.Join(root, path), "SENSITIVE_MARKER")
	}
	mustWrite(t, filepath.Join(root, "go.mod"), "module SENSITIVE_MODULE\n")
	mustWrite(t, filepath.Join(root, ".gitignore"), "main.go\n")
	if _, err := LoadInputs(r, root, false, 2<<20, 256<<20); err != nil {
		t.Fatal(err)
	}
	options.Root = root
	again, err := Compile(options)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.SnapshotID()
	b, _ := again.SnapshotID()
	if a != b {
		t.Fatal("outside-scope changes affected identity")
	}
	encoded, _ := json.Marshal(again)
	if bytes.Contains(encoded, []byte("SENSITIVE")) || bytes.Contains(encoded, []byte("secret.go")) {
		t.Fatal("denied metadata leaked")
	}
	if again.Inputs.GoMod.State != "unavailable" {
		t.Fatal("denied module read")
	}
	// Unlike index-free discovery, compilation deliberately ignores .gitignore.
	if len(again.Files) != 1 {
		t.Fatal("repository ignore file changed compilation")
	}
}

func TestM2ReproducibleIdentitiesAndProfileSeparation(t *testing.T) {
	root, r := inputFixture(t, Options{})
	relocated, copy := inputFixture(t, Options{})
	a, _ := r.SnapshotID()
	b, _ := copy.SnapshotID()
	if a != b {
		t.Fatal("relocation changed canonical index")
	}
	if _, err := LoadInputs(r, relocated, false, 2<<20, 256<<20); err != nil {
		t.Fatal(err)
	}
	other, err := Compile(Options{Root: root, MaxEntries: 99999})
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := r.Inputs.SourceID()
	s2, _ := other.Inputs.SourceID()
	p1, _ := r.Inputs.ProfileID()
	p2, _ := other.Inputs.ProfileID()
	if s1 != s2 || p1 == p2 {
		t.Fatal("source and compiler profile identities conflated")
	}
	other.Inputs.Profile.Compiler = "unknown-provider"
	if _, err := LoadInputs(other, root, false, 2<<20, 256<<20); err == nil {
		t.Fatal("incompatible compiler reused")
	}
}

func TestM2ResourceLimitsFailClosed(t *testing.T) {
	root, r := inputFixture(t, Options{})
	if _, err := Compile(Options{Root: root, MaxReadBytes: 2 * r.Inputs.Sources[0].Bytes}); err != nil {
		t.Fatal("exact total byte cap rejected", err)
	}
	for _, options := range []Options{
		{Root: root, MaxBytes: 1}, {Root: root, MaxReadBytes: 1},
		{Root: root, MaxReadBytes: r.Inputs.Sources[0].Bytes + 1},
		{Root: root, MaxEntries: -1}, {Root: root, MaxReadBytes: 257 << 20},
	} {
		if _, err := Compile(options); err == nil {
			t.Fatalf("resource limit ignored: %+v", options)
		}
	}
	mustWrite(t, filepath.Join(root, "extra.txt"), "outside source inventory but enumeration is bounded")
	if _, err := Compile(Options{Root: root, MaxEntries: 1}); err == nil {
		t.Fatal("entry bound ignored")
	}
	if _, err := LoadInputs(r, root, false, 2<<20, 1); err == nil {
		t.Fatal("serving read limit ignored")
	}
}

func TestM2MalformedManifestRejected(t *testing.T) {
	_, r := inputFixture(t, Options{})
	mutations := map[string]func(*ir.Repository){
		"missing":        func(r *ir.Repository) { r.Inputs = nil },
		"hash":           func(r *ir.Repository) { r.Inputs.Sources[0].SHA256 = strings.Repeat("0", 64) },
		"inventory":      func(r *ir.Repository) { r.Inputs.Sources = nil },
		"path":           func(r *ir.Repository) { r.Inputs.Profile.Allow = []string{"../escape"} },
		"limit":          func(r *ir.Repository) { r.Inputs.Profile.MaxEntries = 100001 },
		"scope":          func(r *ir.Repository) { r.Inputs.Profile.Deny = []string{"main.go"} },
		"absent hash":    func(r *ir.Repository) { r.Inputs.GoMod.SHA256 = strings.Repeat("0", 64) },
		"denied present": func(r *ir.Repository) { r.Inputs.Profile.Deny = []string{"go.mod"} },
	}
	data, _ := json.Marshal(r)
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var bad ir.Repository
			if err := json.Unmarshal(data, &bad); err != nil {
				t.Fatal(err)
			}
			mutate(&bad)
			if err := bad.Validate(); err == nil {
				t.Fatal("malformed manifest accepted")
			}
		})
	}
}

func TestM2AtomicPublicationPreservesPreviousGeneration(t *testing.T) {
	root, r := inputFixture(t, Options{})
	for _, name := range []string{"index.json", "index.json.gz"} {
		path := filepath.Join(root, name)
		if err := Write(r, path, false); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		bad := *r
		bad.Inputs = nil
		if err := Write(&bad, path, false); err == nil {
			t.Fatal("invalid generation published")
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("prior generation damaged")
		}
		if _, err := Read(path); err != nil {
			t.Fatal(err)
		}
	}
	// A rename failure must also leave its target intact and remove the temp.
	target := filepath.Join(root, "existing-directory")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Write(r, target, false); err == nil {
		t.Fatal("directory overwritten")
	}
	leftovers, err := filepath.Glob(filepath.Join(root, ".repoctx-index-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("temporary publication leaked", leftovers, err)
	}
}

func TestM2ConfinedInventoryAndAuxiliarySymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privileges")
	}
	root, r := inputFixture(t, Options{})
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "go.mod"), "module SECRET_OUTSIDE\n")
	mustWrite(t, filepath.Join(outside, "p.go"), "package SECRET_OUTSIDE\n")
	if err := os.Symlink(filepath.Join(outside, "go.mod"), filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(Options{Root: root}); err == nil {
		t.Fatal("auxiliary symlink read or treated as absent")
	}
	if _, err := Compile(Options{Root: root, DenyPaths: []string{"go.mod"}}); err != nil {
		t.Fatal("denied auxiliary input touched", err)
	}
	if _, err := LoadInputs(r, root, false, 2<<20, 256<<20); err == nil {
		t.Fatal("auxiliary symlink accepted on refresh")
	}
	if runtime.GOOS != "linux" {
		return
	}
	// Swap an inventoried directory for a symlink between enumeration and read.
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "pkg", "p.go"), "package p\n")
	base, err := sourceroot.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	paths, err := inventory(base, r.Inputs.Profile)
	if err != nil || len(paths) != 2 {
		t.Fatal(paths, err)
	}
	if err := os.Rename(filepath.Join(root, "pkg"), filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "pkg")); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Read("pkg/p.go", 1<<20); err == nil {
		t.Fatal("directory-swap symlink escaped root")
	}
}

func TestM2PostCompilationVerificationRejectsMixedGeneration(t *testing.T) {
	root, r := inputFixture(t, Options{})
	base, err := sourceroot.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	// Simulate a writer changing an input after capture/parse but before the
	// final validation used by Compile. No partial generation may be returned.
	mustWrite(t, filepath.Join(root, "main.go"), "package main\nfunc NewGeneration() {}\n")
	remaining := int64(256 << 20)
	if err := verifyInputs(base, r.Inputs, &remaining); err == nil {
		t.Fatal("mixed compilation generation accepted")
	}
}
