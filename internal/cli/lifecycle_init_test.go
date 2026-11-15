package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitWritesConfigAndInstalls(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, h.project)
	if !ok || cfg.Language != "alpha" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v (present %v)", cfg, ok)
	}
	pm := h.pm(t, "alpha-one")
	if len(pm.Calls) != 1 || pm.Calls[0] != core.CapInstall || pm.Requests[0].Dir != h.project {
		t.Errorf("calls = %v", pm.Calls)
	}
	var out struct {
		Data initSummary
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Data.ConfigWritten || !out.Data.Installed || out.Data.Source != sourceDetected {
		t.Errorf("summary = %+v", out.Data)
	}
}

func TestInitKeepsExistingChoice(t *testing.T) {
	h := newHarness(t)
	body := "package_manager = \"alpha-two\"\n"
	h.writeProjectConfig(t, h.project, body)
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	got, _ := os.ReadFile(filepath.Join(h.project, config.DirName, config.FileName))
	if string(got) != body {
		t.Errorf("config changed to %q", got)
	}
	if len(h.pm(t, "alpha-two").Calls) != 1 {
		t.Error("install must run through the configured manager")
	}
}

func TestInitFillsConfigWithoutStackAfterConfirmation(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "project_type = \"gadget\"\n")
	if code := h.run("init"); code == core.ExitOK {
		t.Fatal("non-interactive without --yes must not overwrite silently")
	}
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	if cfg.ProjectType != "gadget" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v, want type preserved and manager added", cfg)
	}
}

func TestInitDoesNotTouchManifests(t *testing.T) {
	h := newHarness(t)
	marker := filepath.Join(h.project, "alpha-one.marker")
	before, _ := os.ReadFile(marker)
	h.run("init", "--yes")
	after, _ := os.ReadFile(marker)
	if string(before) != string(after) {
		t.Error("init modified a project file")
	}
	entries, _ := os.ReadDir(h.project)
	for _, e := range entries {
		if e.Name() != "alpha-one.marker" && e.Name() != config.DirName && e.Name() != ".gitignore" {
			t.Errorf("init created %q", e.Name())
		}
	}
}

func TestInitDryRunChangesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, h.project); ok {
		t.Error("dry run wrote config")
	}
	if !h.pm(t, "alpha-one").Requests[0].DryRun {
		t.Error("install must be a dry run")
	}
}

func TestInitInEmptyDirectoryWalksThroughChoicesWithYes(t *testing.T) {
	h := newHarness(t)
	empty := t.TempDir()
	h.deps.Getwd = func() (string, error) { return empty, nil }
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, empty)
	if !ok || cfg.Language != "alpha" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v", cfg)
	}
	if len(h.pm(t, "alpha-one").Calls) != 0 {
		t.Error("nothing exists to install yet")
	}
	if !strings.Contains(h.stdout.String(), "upc new") {
		t.Errorf("stdout = %q should point to `upc new`", h.stdout)
	}
}

func TestInitInEmptyDirectoryNonInteractiveFails(t *testing.T) {
	h := newHarness(t)
	empty := t.TempDir()
	h.deps.Getwd = func() (string, error) { return empty, nil }
	if code := h.run("init"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, empty); ok {
		t.Error("nothing may be written when the walkthrough cannot run")
	}
}

func TestInitRefusesIncompatibleConfig(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "language = \"alpha\"\npackage_manager = \"beta-one\"\n")
	if code := h.run("init", "--yes"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
}
