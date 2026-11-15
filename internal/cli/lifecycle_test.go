package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func (h *harness) writeProjectConfig(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, config.DirName, config.FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, dir string) (config.Config, bool) {
	t.Helper()
	store := config.Store{FS: fsx.OS{}, GlobalDir: filepath.Join(dir, "no-global"), ProjectRoot: dir}
	if _, err := os.Stat(filepath.Join(dir, config.DirName, config.FileName)); err != nil {
		return config.Config{}, false
	}
	c, err := store.LoadScope(config.Project)
	if err != nil {
		t.Fatal(err)
	}
	return c, true
}

func TestNewYesPicksDefaultsAndWritesProjectConfig(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "thing", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, filepath.Join(h.project, "thing"))
	want := config.Config{Language: "alpha", PackageManager: "alpha-one", ProjectType: "gadget"}
	if !ok || cfg.Language != want.Language || cfg.PackageManager != want.PackageManager || cfg.ProjectType != want.ProjectType {
		t.Errorf("config = %+v (present %v), want %+v", cfg, ok, want)
	}
	if !strings.Contains(h.stdout.String(), "scaffolded thing") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestNewAcceptsNameFlagAndExplicitStack(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "--name", "x", "--language", "beta"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "x")); cfg.PackageManager != "beta-one" {
		t.Errorf("config = %+v", cfg)
	}
	if code := h.run("new", "gadget", "y", "--pm", "alpha-two"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "y")); cfg.Language != "alpha" || cfg.PackageManager != "alpha-two" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestNewUsesConfiguredStack(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "language = \"beta\"\n")
	if code := h.run("new", "gadget", "z"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "z")); cfg.Language != "beta" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestNewUsageErrors(t *testing.T) {
	h := newHarness(t)
	cases := map[string][]string{
		"unknown type":            {"new", "nonsense", "x", "--yes"},
		"missing name":            {"new", "gadget", "--yes"},
		"no type":                 {"new"},
		"name twice":              {"new", "gadget", "a", "--name", "b", "--yes"},
		"language/pm mismatch":    {"new", "gadget", "x", "--language", "beta", "--pm", "alpha-one"},
		"unknown language":        {"new", "gadget", "x", "--language", "cobol"},
		"unknown pm":              {"new", "gadget", "x", "--pm", "nope"},
		"ambiguous non-interact.": {"new", "gadget", "x"},
		"global scope":            {"new", "gadget", "x", "--yes", "--global"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code := h.run(args...); code != core.ExitUsage {
				t.Errorf("%v: exit %d, stderr %q", args, code, h.stderr)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(h.project, "x")); err == nil {
		t.Error("a refused command must not create the project")
	}
}

func TestNewAmbiguityMessageNamesTheFlag(t *testing.T) {
	h := newHarness(t)
	h.run("new", "gadget", "x")
	if !strings.Contains(h.stderr.String(), "--language") {
		t.Errorf("stderr = %q, want a hint to pass --language", h.stderr)
	}
}

func TestNewDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "thing", "--yes", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.project, "thing")); err == nil {
		t.Error("dry run created the project")
	}
}
