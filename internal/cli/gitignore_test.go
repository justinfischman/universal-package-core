package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func readIgnore(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitCreatesGitignoreWithRunDir(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if got := readIgnore(t, h.project); got != ".universal-package-core/run/\n" {
		t.Errorf(".gitignore = %q", got)
	}
}

func TestInitAppendsOnceAndKeepsContent(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(h.project, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules/\n.env"), 0o644); err != nil { // no trailing newline
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if code := h.run("init", "--yes"); code != core.ExitOK {
			t.Fatalf("run %d: exit %d: %s", i, code, h.stderr)
		}
	}
	got := readIgnore(t, h.project)
	if got != "node_modules/\n.env\n.universal-package-core/run/\n" {
		t.Errorf(".gitignore = %q", got)
	}
	if strings.Count(got, "run") != 1 {
		t.Errorf("entry duplicated: %q", got)
	}
}

func TestInitRecognizesEquivalentEntries(t *testing.T) {
	for _, body := range []string{
		".universal-package-core/run/\n", "/.universal-package-core/run\n", "  .universal-package-core/  \n",
	} {
		h := newHarness(t)
		path := filepath.Join(h.project, ".gitignore")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := h.run("init", "--yes"); code != core.ExitOK {
			t.Fatalf("exit %d: %s", code, h.stderr)
		}
		if got := readIgnore(t, h.project); got != body {
			t.Errorf("%q was modified to %q", body, got)
		}
	}
}

func TestInitDryRunDoesNotWriteGitignore(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.project, ".gitignore")); err == nil {
		t.Error("dry run wrote .gitignore")
	}
}
