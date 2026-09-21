package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
)

func TestRootAndLegacyCommandHelpMatch(t *testing.T) {
	root := runCommand(t, ".", "help")
	legacy := runCommand(t, "./cmd/repoctx", "help")
	if !bytes.Equal(root, legacy) {
		t.Fatalf("root and ./cmd/repoctx help output differ:\nroot:\n%s\nlegacy:\n%s", root, legacy)
	}
}

func TestRootAndLegacyVersionMatch(t *testing.T) {
	root := runCommand(t, ".", "version", "-format", "json")
	legacy := runCommand(t, "./cmd/repoctx", "version", "-format", "json")
	if !bytes.Equal(root, legacy) {
		t.Fatalf("root and ./cmd/repoctx version output differ:\nroot:\n%s\nlegacy:\n%s", root, legacy)
	}
	var info struct {
		Version      string `json:"version"`
		Program      string `json:"program"`
		Release      string `json:"release"`
		Revision     string `json:"revision"`
		Modified     string `json:"modified"`
		Distribution string `json:"distribution"`
		Contracts    struct {
			IR        string `json:"ir"`
			Context   string `json:"context"`
			Discovery string `json:"discovery"`
			Artifacts string `json:"artifacts"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(root, &info); err != nil {
		t.Fatal(err)
	}
	if info.Version != "repoctx.build/v1alpha1" || info.Program != "repoctx" ||
		info.Release == "" || info.Revision == "" || info.Modified == "" ||
		info.Distribution != "go" || info.Contracts.IR == "" ||
		info.Contracts.Context == "" || info.Contracts.Discovery == "" ||
		info.Contracts.Artifacts == "" {
		t.Fatalf("incomplete version output: %+v", info)
	}
	if got := runCommand(t, ".", "--version"); !bytes.HasPrefix(got, []byte("repoctx ")) {
		t.Fatalf("--version output = %q", got)
	}
}

func runCommand(t *testing.T, target string, args ...string) []byte {
	t.Helper()
	command := []string{"run", "-buildvcs=false", target}
	command = append(command, args...)
	cmd := exec.Command("go", command...)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run %s %v: %v\n%s", target, args, err, out)
	}
	return out
}
