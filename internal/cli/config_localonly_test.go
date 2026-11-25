package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// TestConfigGetAgreesWithShowOriginForLocalOnly covers a project whose only
// config file is local.toml: get must resolve the value show --origin finds,
// and show --origin must still mask it.
func TestConfigGetAgreesWithShowOriginForLocalOnly(t *testing.T) {
	h := newHarness(t)
	h.writeLayer(t, filepath.Join(h.project, config.DirName), config.LocalName, "language = \"custom-lang\"\n")

	if code := h.run("config", "get", "language"); code != core.ExitOK {
		t.Fatalf("get exit %d: %s", code, h.stderr)
	}
	if got, want := h.stdout.String(), "language = custom-lang (project)\n"; got != want {
		t.Errorf("get stdout = %q, want %q", got, want)
	}

	h.stdout.Reset()
	if code := h.run("config", "show", "--origin"); code != core.ExitOK {
		t.Fatalf("show exit %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	if !strings.Contains(out, "language = **** (app local)\n") {
		t.Errorf("show --origin = %q, want masked app local language", out)
	}
	if strings.Contains(out, "custom-lang") {
		t.Errorf("local value leaked in show --origin: %q", out)
	}
}
