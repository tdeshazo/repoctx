package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/discovery"
)

func TestDiscoveryCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.yaml"), []byte("retry: 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		op   string
		args []string
	}{
		{op: "overview"}, {op: "files", args: []string{"-glob", "*.yaml"}},
		{op: "search", args: []string{"-query", "retry"}},
		{op: "discover", args: []string{"-query", "retry configuration"}},
		{op: "read", args: []string{"-file", "settings.yaml:1:1"}},
	} {
		t.Run(tc.op, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "result.json")
			args := append([]string{"-root", root, "-o", out}, tc.args...)
			if err := runDiscovery(context.Background(), tc.op, args); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var response discovery.Response
			if err := json.Unmarshal(b, &response); err != nil {
				t.Fatal(err)
			}
			if response.Version != discovery.Version {
				t.Fatal(response.Version)
			}
			if tc.op != "overview" && len(response.Results) != 1 {
				t.Fatal("missing result", string(b))
			}
		})
	}
}

func TestDiscoveryUsageAndAtomicOutput(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "existing.json")
	if err := os.WriteFile(out, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-query", "x", "extra"}, {"-query", "[", "-regex"}, {"-unknown"}, {},
	} {
		err := runDiscovery(context.Background(), "search", args)
		var usage *discovery.UsageError
		if !errors.As(err, &usage) {
			t.Fatalf("expected usage error for %v: %v", args, err)
		}
	}
	err := runDiscovery(context.Background(), "overview", []string{"-root", root, "-o", out, "-max-bytes", "1"})
	if err == nil {
		t.Fatal("expected budget error")
	}
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "preserve" {
		t.Fatal("failed command replaced output")
	}
}
