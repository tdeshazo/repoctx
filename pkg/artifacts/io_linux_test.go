package artifacts

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Linux inotify observes actual accesses rather than assuming that permission
// errors imply no reads (the test may run as a privileged user).
func TestDeclaredPathsCauseNoReadsOrExecution(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "evidence")
	if err := os.WriteFile(source, []byte("untrusted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "denied"), []byte("private"), 0000); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "EXECUTED")
	script := filepath.Join(root, "check")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := syscall.Close(fd); err != nil {
			t.Error(err)
		}
	}()
	if _, err := syscall.InotifyAddWatch(fd, root, syscall.IN_OPEN|syscall.IN_ACCESS); err != nil {
		t.Fatal(err)
	}
	// Relative paths actually address the watched directory from the package cwd.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Enter the watched directory so a mistaken implicit read hits the watch.
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	}()
	c := minimal()
	a := &c.Artifacts[0]
	a.Sources[0].Path = "evidence"
	a.DeclaredInputs = []Input{{Path: "evidence", Role: "source", Required: true}, {Path: "missing", Role: "configuration", Required: false}, {Path: "denied", Role: "source", Required: true}}
	a.Kind = "verification_obligation"
	id := "check"
	a.RunnerCheckID = &id
	accepted(t, wire(t, c), Limits{})
	buf := make([]byte, 4096)
	n, err := syscall.Read(fd, buf)
	if n > 0 || !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("declaration validation accessed watched paths: n=%d err=%v", n, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("opaque check ID executed")
	}
}
