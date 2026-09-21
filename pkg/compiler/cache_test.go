package compiler

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tdeshazo/repoctx/internal/benchfixture"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

func TestIncrementalParseFragmentsMatchCleanCompilation(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.test/repo\n\ngo 1.23\n")
	mustWrite(t, filepath.Join(root, "a", "a.go"), `package a
import "example.test/repo/b"
func Call() int { return b.Value() }
`)
	mustWrite(t, filepath.Join(root, "b", "b.go"), "package b\nfunc Value() int { return 1 }\n")
	mustWrite(t, filepath.Join(root, "docs", "guide.md"), "# Guide\n\nSee [API](api.md).\n")

	cold, coldStats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if coldStats.Misses != 3 || coldStats.Hits != 0 || coldStats.WrittenBytes == 0 {
		t.Fatalf("cold stats = %+v", coldStats)
	}
	assertMatchesClean(t, root, cold)

	warm, warmStats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if warmStats.Hits != 3 || warmStats.Misses != 0 || warmStats.WrittenBytes != 0 {
		t.Fatalf("warm stats = %+v", warmStats)
	}
	assertSameRepository(t, cold, warm)

	mustWrite(t, filepath.Join(root, "b", "b.go"), "package b\nfunc Value() int { return 2 }\n")
	modified, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Hits != 2 || stats.Misses != 1 {
		t.Fatalf("modified stats = %+v", stats)
	}
	assertMatchesClean(t, root, modified)

	mustWrite(t, filepath.Join(root, "c", "c.go"), "package c\nfunc Added() {}\n")
	added, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Hits != 3 || stats.Misses != 1 {
		t.Fatalf("addition stats = %+v", stats)
	}
	assertMatchesClean(t, root, added)

	if err := os.Rename(filepath.Join(root, "docs", "guide.md"),
		filepath.Join(root, "docs", "renamed.md")); err != nil {
		t.Fatal(err)
	}
	renamed, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Hits != 4 || stats.Misses != 0 {
		t.Fatalf("rename stats = %+v", stats)
	}
	assertMatchesClean(t, root, renamed)

	mustWrite(t, filepath.Join(root, "go.mod"), "module example.test/changed\n\ngo 1.23\n")
	configured, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Hits != 4 || stats.Misses != 0 {
		t.Fatalf("configuration stats = %+v", stats)
	}
	assertMatchesClean(t, root, configured)

	if err := os.Remove(filepath.Join(root, "c", "c.go")); err != nil {
		t.Fatal(err)
	}
	deleted, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Hits != 3 || stats.Misses != 0 || len(deleted.Files) != 3 {
		t.Fatalf("deletion stats = %+v; files=%d", stats, len(deleted.Files))
	}
	assertMatchesClean(t, root, deleted)

	profiled, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache,
		MaxEntries: 99999})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Misses != 3 || stats.Hits != 0 {
		t.Fatalf("profile change reused fragments: %+v", stats)
	}
	cleanProfiled, err := Compile(Options{Root: root, MaxEntries: 99999})
	if err != nil {
		t.Fatal(err)
	}
	assertSameRepository(t, cleanProfiled, profiled)
}

func TestIncrementalCacheRejectsOrRepairsUnsafeEntries(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "main.go"), "package main\nfunc main() {}\n")
	inside := filepath.Join(root, ".repoctx-cache")
	if _, _, err := CompileWithStats(Options{Root: root, CacheDir: inside}); err == nil {
		t.Fatal("cache inside indexed repository accepted")
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("rejected in-repository cache was created: %v", err)
	}

	cache := t.TempDir()
	first, _, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := filepath.Glob(filepath.Join(cache, "fragments", "*", "*.json"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache entries = %v, %v", entries, err)
	}
	if err := os.WriteFile(entries[0], []byte("{corrupt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repaired, stats, err := CompileWithStats(Options{Root: root, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Invalid != 1 || stats.Misses != 1 || stats.Hits != 0 {
		t.Fatalf("corrupt entry stats = %+v", stats)
	}
	assertSameRepository(t, first, repaired)
	if _, _, err := CompileWithStats(Options{Root: root, CacheDir: cache}); err != nil {
		t.Fatal("repaired entry was not reusable", err)
	}

	if runtime.GOOS != "windows" {
		insecure := t.TempDir()
		if err := os.Chmod(insecure, 0o777); err != nil {
			t.Fatal(err)
		}
		if _, _, err := CompileWithStats(Options{Root: root, CacheDir: insecure}); err == nil {
			t.Fatal("world-writable cache accepted")
		}
	}
}

func TestIncrementalCacheStorageBound(t *testing.T) {
	root := t.TempDir()
	if _, err := benchfixture.Write(root, 100); err != nil {
		t.Fatal(err)
	}
	repo, stats, err := CompileWithStats(Options{Root: root, CacheDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ReferencedBytes*4 > int64(len(canonical))*5 {
		t.Fatalf("referenced cache is %.3fx canonical IR; limit is 1.25x",
			float64(stats.ReferencedBytes)/float64(len(canonical)))
	}
}

func assertMatchesClean(t *testing.T, root string, incremental *ir.Repository) {
	t.Helper()
	clean, err := Compile(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	assertSameRepository(t, clean, incremental)
}

func assertSameRepository(t *testing.T, expected, actual *ir.Repository) {
	t.Helper()
	want, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("incremental output differs from clean output\nwant: %s\ngot:  %s", want, got)
	}
	wantSnapshot, err := expected.SnapshotID()
	if err != nil {
		t.Fatal(err)
	}
	gotSnapshot, err := actual.SnapshotID()
	if err != nil {
		t.Fatal(err)
	}
	if wantSnapshot != gotSnapshot {
		t.Fatalf("snapshot mismatch: %s != %s", wantSnapshot, gotSnapshot)
	}
}
