package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestWorkspaceCheckExpandsNativeGlobsBeforeComparing(t *testing.T) {
	h, _ := contractHarness(t)
	h.writeProjectConfig(t, h.project, "[workspace]\nmembers = [\"apps/api\", \"apps/web\"]\n"+contractAppConfig)
	for _, d := range []string{"apps/api", "apps/web", "apps/old"} {
		if err := os.MkdirAll(filepath.Join(h.project, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h.pm(t, "alpha-one").Members = []string{"apps/*", "!apps/old"}
	if code := h.run("workspace", "check", "--update"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "members: ok") {
		t.Errorf("output = %s", h.stdout.String())
	}
	h.pm(t, "alpha-one").Members = []string{"apps/*"}
	if code := h.run("workspace", "check"); code == core.ExitOK {
		t.Fatal("want failure: apps/old is native-only")
	}
	if !strings.Contains(h.stdout.String(), "apps/old") {
		t.Errorf("output lacks apps/old: %s", h.stdout.String())
	}
}
