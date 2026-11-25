package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// exportSource is a RunSource and RecipeExporter writing "recipes.file"; the
// real exporter has its own tests (arch rule: cli may not import concrete modules).
type exportSource struct{ fakeSource }

func (exportSource) Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "recipes.file"))
	return err == nil
}

func (exportSource) RecipeFile(program string, verbs []string) core.RecipeFile {
	return core.RecipeFile{Name: "recipes.file", Body: program + ": " + strings.Join(verbs, ",") + "\n"}
}

var _ core.RecipeExporter = exportSource{}

func exportHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.deps.Registries.RunSources = []core.RunSource{exportSource{}}
	return h
}

func TestExportWritesRecipeFileAndKeepsAnExistingOne(t *testing.T) {
	h := exportHarness(t)
	if code := h.run("export"); code != core.ExitOK {
		t.Fatalf("export exit %d: %s", code, h.stderr.String())
	}
	path := filepath.Join(h.project, "recipes.file")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "install,build,test,lint,audit,outdated,run,up,down") {
		t.Errorf("recipe file = %q", data)
	}
	if code := h.run("export"); code != core.ExitUsage {
		t.Errorf("second export exit %d, want usage error", code)
	}
	if err := os.WriteFile(path, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("export", "--force"); code != core.ExitOK {
		t.Fatalf("export --force exit %d: %s", code, h.stderr.String())
	}
	if data, _ := os.ReadFile(path); string(data) == "mine\n" {
		t.Error("--force did not overwrite")
	}
}

func TestExportDryRunWritesNothing(t *testing.T) {
	h := exportHarness(t)
	if code := h.run("--dry-run", "export"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(h.project, "recipes.file")); !os.IsNotExist(err) {
		t.Errorf("dry run wrote the file: %v", err)
	}
	if !strings.Contains(h.stdout.String(), "dry run") {
		t.Errorf("stdout = %q", h.stdout.String())
	}
}

func TestExportWithoutAnExporterIsRefused(t *testing.T) {
	h := newHarness(t)
	h.deps.Registries.RunSources = []core.RunSource{fakeSource{}}
	if code := h.run("export"); code == core.ExitOK {
		t.Error("want a failure when no source can export")
	}
}
