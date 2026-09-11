package agentctx

import (
	"strings"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compiler"
)

func TestM2TaskIdentityBindsSelectionAndRendering(t *testing.T) {
	root, r := goFixture(t)
	o := baseOptions(root)
	first, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bundle.Snapshot.SourceID == first.Bundle.Snapshot.ID || first.Bundle.Snapshot.ProfileID == "" {
		t.Fatal("identities not separated")
	}
	again, err := Build(r, o)
	if err != nil || first.Bundle.TaskID != again.Bundle.TaskID {
		t.Fatal("unstable task identity", err)
	}
	mutations := map[string]func(*Options){
		"query":      func(o *Options) { o.Query = "Ping function" },
		"renderer":   func(o *Options) { o.Format = "markdown" },
		"budget":     func(o *Options) { o.MaxBytes++ },
		"selection":  func(o *Options) { o.MaxUnits = 1 },
		"read limit": func(o *Options) { o.MaxReadBytes = 32 << 20 },
		"tokenizer": func(o *Options) {
			o.TokenizerID = "test/bytes/v1"
			o.CountTokens = func(b []byte) (int, error) { return len(b), nil }
		},
		"explicit symbol": func(o *Options) { o.Symbols = []string{first.Bundle.Symbols[0].ID} },
		"explicit unit":   func(o *Options) { o.Units = []string{first.Bundle.Units[0].ID} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := o
			mutate(&changed)
			res, err := Build(r, changed)
			if err != nil {
				t.Fatal(err)
			}
			if res.Bundle.TaskID == first.Bundle.TaskID {
				t.Fatal("task cache identity crossed changed settings")
			}
		})
	}
	// Root locations are not part of canonical task identity.
	if err := normalize(&o); err != nil {
		t.Fatal(err)
	}
	a, _ := taskID(first.Bundle.Snapshot.ID, o)
	o.Root = "/different/host/location"
	b, _ := taskID(first.Bundle.Snapshot.ID, o)
	if a != b {
		t.Fatal("host path entered task identity")
	}
}

func TestM2ImmutableModeRequiresCallerPinAndLabelsTrust(t *testing.T) {
	root, r := goFixture(t)
	o := baseOptions(root)
	o.Consistency = "immutable"
	if _, err := Build(r, o); err == nil {
		t.Fatal("unbound immutable assertion accepted")
	}
	o.ExpectedSnapshot, _ = r.SnapshotID()
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if res.Bundle.Snapshot.Verification != "immutable" || !strings.Contains(string(res.Payload), "caller") {
		t.Fatal("immutable verification misrepresented")
	}
	// This deliberately violates the caller assertion to check the documented
	// boundary: immutable reuse skips inventory, verified-local does not.
	write(t, root, "new.md", "# Added\n")
	if _, err := Build(r, o); err != nil {
		t.Fatal("immutable mode unexpectedly scans inventory", err)
	}
	o.Consistency = "verified-local"
	if _, err := Build(r, o); err == nil {
		t.Fatal("verified-local missed new input")
	}
	o.Consistency = "immutable"
	write(t, root, "main.go", "package changed\nfunc Ping() {}\n")
	if _, err := Build(r, o); err == nil {
		t.Fatal("immutable mode skipped source hash checks")
	}
}

func TestM2ScopeCompatibilityBeforeReads(t *testing.T) {
	root, r := goFixture(t)
	write(t, root, "go.mod", "module private.example\n")
	r, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	o := baseOptions(root)
	o.DenyPaths = []string{"go.mod"}
	if res, err := Build(r, o); err == nil || res != nil {
		t.Fatal("auxiliary-derived metadata reused under deny")
	}
	r, err = compiler.Compile(compiler.Options{Root: root, DenyPaths: o.DenyPaths})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Build(r, o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(res.Payload), "private.example") {
		t.Fatal("module metadata leaked")
	}
	if !strings.Contains(string(res.Payload), "go_module_metadata_policy_denied") {
		t.Fatal("unavailable capability omitted")
	}
	o.DenyPaths = nil // Trust the authenticated index's declared policy.
	again, err := Build(r, o)
	if err != nil || again.Bundle.TaskID != res.Bundle.TaskID {
		t.Fatal("effective policy is not canonical", err)
	}
	o.AllowPaths = []string{"main.go"}
	if _, err := Build(r, o); err == nil {
		t.Fatal("incompatible scope reused")
	}
}

func TestM2UnknownTokenizerIdentityRejected(t *testing.T) {
	root, r := goFixture(t)
	o := baseOptions(root)
	o.CountTokens = func(b []byte) (int, error) { return len(b), nil }
	if _, err := Build(r, o); err == nil {
		t.Fatal("unidentified tokenizer entered task cache key")
	}
}
