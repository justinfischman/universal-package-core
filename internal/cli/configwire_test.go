package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func (h *harness) writeConfig(t *testing.T, body string) {
	t.Helper()
	dir := filepath.Join(h.project, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunConfigScriptOutranksManagerAndPassesArgs(t *testing.T) {
	h, f := wired(t) // the fake manager would accept "build" from its manifest
	h.writeConfig(t, "[scripts]\nbuild = [\"mybuild\", \"--fast\"]\n")
	if code := h.run("run", "build", "extra"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if want := []string{"mybuild --fast extra"}; !reflect.DeepEqual(callNames(f), want) {
		t.Errorf("calls = %v, want %v", callNames(f), want)
	}
	if f.Calls[0].Dir != h.project {
		t.Errorf("dir = %q, want %q", f.Calls[0].Dir, h.project)
	}
}

func TestRunUnconfiguredScriptStillUsesManager(t *testing.T) {
	h, f := wired(t)
	h.writeConfig(t, "[scripts]\nother = [\"x\"]\n")
	if code := h.run("run", "build"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(f.Calls) != 0 {
		t.Errorf("config does not define build; manager answers: %v", callNames(f))
	}
}

func TestRunConfigScriptDryRunDoesNotExecute(t *testing.T) {
	h, f := wired(t)
	h.writeConfig(t, "[scripts]\nbuild = [\"mybuild\"]\n")
	if code := h.run("--dry-run", "run", "build"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(f.Calls) != 0 {
		t.Errorf("dry run executed: %v", callNames(f))
	}
}

func TestUpConfiguredCommandOutranksScriptsAndProviders(t *testing.T) {
	h, f := wired(t)
	h.writeProjectFile(t, "stack.file")
	h.writeConfig(t, "up = [\"srv\", \"--port\", \"8080\"]\n")
	if code := h.run("up"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if want := []string{"srv --port 8080"}; !reflect.DeepEqual(callNames(f), want) {
		t.Fatalf("calls = %v, want %v", callNames(f), want)
	}
	if len(f.Procs) != 1 || f.Procs[0].Cmd.Dir != h.project {
		t.Fatalf("procs = %+v", f.Procs)
	}
	if _, err := os.Stat(filepath.Join(h.project, ".universal-package-core", "run", "up.pid")); err != nil {
		t.Errorf("pid not recorded: %v", err)
	}
	if code := h.run("up"); code == core.ExitOK {
		t.Error("a second up while running must fail")
	}
}
