package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceDenyBeforeIndexing(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "p.go"), "package p\nfunc Public(){}\n")
	mustWrite(t, filepath.Join(root, "secret", "s.go"), "package secret\nconst Private = \"MARKER_NEVER_INDEX\"\n")
	r, e := Compile(Options{Root: root, DenyPaths: []string{"secret"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Files) != 1 {
		t.Fatal("denied file indexed")
	}
	for _, s := range r.Strings {
		if strings.Contains(s, "MARKER_NEVER_INDEX") {
			t.Fatal("secret indexed")
		}
	}
}
func TestGoCrossFileGenericReceiverParent(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "types.go"), "package p\ntype Client struct{}\ntype Box[T any] struct{}\n")
	mustWrite(t, filepath.Join(root, "methods.go"), "package p\nfunc (c *Client) Run(){}\nfunc (b *Box[T]) Get(){}\n")
	r, e := Compile(Options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	for _, s := range r.Symbols {
		ids[r.String(s.ID)] = true
	}
	if !ids["go:p#Client.Run"] || !ids["go:p#Box.Get"] {
		t.Fatal("cross-file receiver not linked", ids)
	}
}
func TestCrossLanguageCallNotLinked(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "p.go"), "package p\nfunc Secret(){}\n")
	mustWrite(t, filepath.Join(root, "p.py"), "def run():\n    return Secret()\n")
	r, e := Compile(Options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	for _, e := range r.Edges {
		if r.String(e.Text) == "Secret" && e.ToSymbol != 0 {
			t.Fatal("Python linked to Go by spelling")
		}
	}
}
func TestPythonNoInterpreterOrRepositoryExecution(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "EXECUTED")
	// These are deliberately hostile-looking repository modules. Native
	// Tree-sitter parsing must never import or execute them.
	mustWrite(t, filepath.Join(root, "ast.py"), "raise RuntimeError('shadow ast imported')\n")
	mustWrite(t, filepath.Join(root, "json.py"), "open('"+marker+"','w').write('bad')\n")
	mustWrite(t, filepath.Join(root, "p.py"), "def sample():\n    return 1\n")
	r, e := Compile(Options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("repository module executed")
	}
	if len(r.Symbols) == 0 {
		t.Fatal("isolated parser unavailable")
	}
}
func TestReadRejectsTrailingJSON(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "p.go"), "package p\nfunc Ping(){}\n")
	r, e := Compile(Options{Root: root})
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, "r.json")
	if e = Write(r, out, false); e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(out, os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	f.WriteString("{}")
	f.Close()
	if _, e = Read(out); e == nil {
		t.Fatal("trailing JSON accepted")
	}
}
