package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func (h *harness) writeLayer(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// layered builds a global value, a project value and a masked local value.
func (h *harness) layered(t *testing.T) {
	t.Helper()
	h.writeLayer(t, h.home, config.FileName, "language = \"global-lang\"\nproject_type = \"global-type\"\n")
	proj := filepath.Join(h.project, config.DirName)
	h.writeLayer(t, proj, config.FileName, "package_manager = \"alpha-one\"\n")
	h.writeLayer(t, proj, config.LocalName, "project_type = \"s3cret\"\n")
}

func TestConfigShowOriginText(t *testing.T) {
	h := newHarness(t)
	h.layered(t)
	if code := h.run("config", "show", "--origin"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	want := "language = global-lang (global)\n" +
		"package_manager = alpha-one (app config)\n" +
		"package_managers = (unset)\n" +
		"project_type = **** (app local)\n"
	if got := h.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(h.stdout.String(), "s3cret") {
		t.Error("local value leaked")
	}
}

func TestConfigShowOriginJSONGolden(t *testing.T) {
	h := newHarness(t)
	h.layered(t)
	if code := h.run("config", "show", "--origin", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if strings.Contains(h.stdout.String(), "s3cret") {
		t.Error("local value leaked")
	}
	golden(t, "config_show_origin.json", h.stdout.Bytes())
}

func TestConfigShowOriginGlobalIgnoresProject(t *testing.T) {
	h := newHarness(t)
	h.layered(t)
	if code := h.run("config", "show", "--origin", "--global"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	want := "language = global-lang (global)\n" +
		"package_manager = (unset)\n" +
		"package_managers = (unset)\n" +
		"project_type = global-type (global)\n"
	if got := h.stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestConfigShowRequiresOrigin(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "show"); code != core.ExitUsage {
		t.Errorf("exit %d, want usage", code)
	}
}
