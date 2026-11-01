package fsx_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
)

func TestWriteFileAtomicCreatesDirsAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.toml")
	if err := (fsx.OS{}).WriteFileAtomic(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "x = 1\n" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestWriteFileAtomicReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	f := fsx.OS{}
	for _, content := range []string{"one", "two"} {
		if err := f.WriteFileAtomic(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := f.ReadFile(path); string(got) != "two" {
		t.Errorf("got %q", got)
	}
}

func TestWriteFileAtomicFailureLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	// Destination is a non-empty directory, so the final rename must fail.
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(filepath.Join(dest, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (fsx.OS{}).WriteFileAtomic(dest, []byte("data"), 0o644); err == nil {
		t.Fatal("expected error")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "dest" {
			t.Errorf("leftover file %q", e.Name())
		}
	}
}

func TestDirFSAndStat(t *testing.T) {
	dir := t.TempDir()
	f := fsx.OS{}
	if err := f.WriteFileAtomic(filepath.Join(dir, "m"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Stat(filepath.Join(dir, "m")); err != nil {
		t.Error(err)
	}
	if _, err := f.DirFS(dir).Open("m"); err != nil {
		t.Error(err)
	}
}
