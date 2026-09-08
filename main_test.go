package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestRootAndLegacyCommandHelpMatch(t *testing.T) {
	root := runHelp(t, ".")
	legacy := runHelp(t, "./cmd/repoctx")
	if !bytes.Equal(root, legacy) {
		t.Fatalf("root and ./cmd/repoctx help output differ:\nroot:\n%s\nlegacy:\n%s", root, legacy)
	}
}

func runHelp(t *testing.T, target string) []byte {
	t.Helper()
	cmd := exec.Command("go", "run", "-buildvcs=false", target, "help")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run %s help: %v\n%s", target, err, out)
	}
	return out
}
