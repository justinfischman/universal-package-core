package cli

import (
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// The version subcommand and add's --version flag live side by side.
func TestAddVersionFlagDoesNotCollideWithVersionCommand(t *testing.T) {
	h := newHarness(t)
	if code := h.run("version"); code != core.ExitOK {
		t.Fatalf("version exit = %d", code)
	}
	if !strings.Contains(h.stdout.String(), "upc") {
		t.Errorf("version output = %q", h.stdout.String())
	}
}

// A manager that has not implemented core.Versioned must refuse a version
// instead of silently adding the unpinned package.
func TestAddVersionRefusedWhenManagerHasNoDialect(t *testing.T) {
	for _, flag := range []string{"--version", "--constraint"} {
		h := newHarness(t)
		if code := h.run("add", "foo", flag, ">=2.3.4"); code != core.ExitUsage {
			t.Fatalf("%s exit = %d, want %d", flag, code, core.ExitUsage)
		}
		if !strings.Contains(h.stderr.String(), "does not support version constraints") {
			t.Errorf("%s stderr = %q", flag, h.stderr.String())
		}
	}
}

func TestAddVersionAndConstraintMustAgree(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add", "foo", "--version", ">=1", "--constraint", ">=2"); code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, core.ExitUsage)
	}
}
