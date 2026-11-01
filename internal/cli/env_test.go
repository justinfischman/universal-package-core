package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func (h *harness) touch(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.project, name), []byte("SECRET=hunter2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnvListMergesConfigAndConvention(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "default_environment = \"dev\"\n[environments.dev]\nfile = \"config/dev.env\"\n")
	h.touch(t, ".env.local")
	h.touch(t, ".env.example") // a template, not an environment
	if err := os.MkdirAll(filepath.Join(h.project, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.project, "config", "dev.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("env", "list", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	var out struct{ Data envReport }
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range out.Data.Envs {
		got = append(got, e.Name+"="+e.File+"/"+e.Source)
	}
	want := []string{"dev=config/dev.env/config", "local=.env.local/convention"}
	if !reflect.DeepEqual(got, want) || out.Data.Active != "dev" {
		t.Errorf("envs = %v active %q, want %v active dev", got, out.Data.Active, want)
	}
	if strings.Contains(h.stdout.String(), "hunter2") {
		t.Error("env values must never be printed")
	}
}

func TestEnvUsePersistsSelection(t *testing.T) {
	h := newHarness(t)
	h.touch(t, ".env.staging")
	if code := h.run("env", "use", "staging"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, h.project); cfg.ActiveEnvironment != "staging" {
		t.Errorf("config = %+v", cfg)
	}
	if code := h.run("env", "show"); code != core.ExitOK || !strings.Contains(h.stdout.String(), "staging (.env.staging) [convention]") {
		t.Errorf("show: exit %d, stdout %q", code, h.stdout)
	}
}

func TestEnvUseDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	h.touch(t, ".env.staging")
	if code := h.run("env", "use", "staging", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, h.project); ok {
		t.Error("dry run wrote config")
	}
}

func TestEnvUseMissingEnvIsUsageError(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "[environments.dev]\nfile = \".env.dev\"\n")
	for _, name := range []string{"nope", "dev", "../x"} { // unknown, declared but file missing, invalid
		if code := h.run("env", "use", name); code != core.ExitUsage {
			t.Errorf("use %s: exit %d, want %d (%s)", name, code, core.ExitUsage, h.stderr)
		}
	}
	if cfg, _ := readConfig(t, h.project); cfg.ActiveEnvironment != "" {
		t.Errorf("a refused use must not persist: %+v", cfg)
	}
}

func TestEnvShowWithoutSelection(t *testing.T) {
	h := newHarness(t)
	if code := h.run("env", "show"); code != core.ExitOK || !strings.Contains(h.stdout.String(), "no active environment") {
		t.Errorf("exit %d stdout %q", code, h.stdout)
	}
}

func TestDoctorReportsMissingActiveEnv(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "active_environment = \"gone\"\n")
	if code := h.run("doctor", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), `active environment "gone" does not exist`) {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestModuleRunnerSetsUPCEnv(t *testing.T) {
	h := newHarness(t)
	fake := &execx.FakeRunner{}
	h.useRunner(fake)
	run := func() []string {
		fake.Calls = nil
		s, err := newSession(h.deps, Flags{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.moduleRunner()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.Run(context.Background(), execx.Cmd{Name: "tool", Env: []string{"A=1"}}); err != nil {
			t.Fatal(err)
		}
		return fake.Calls[0].Env
	}
	if env := run(); len(env) != 1 || env[0] != "A=1" {
		t.Errorf("no active env: Env = %v", env)
	}
	h.touch(t, ".env.dev")
	if code := h.run("env", "use", "dev"); code != core.ExitOK {
		t.Fatal(h.stderr.String())
	}
	if env := run(); !reflect.DeepEqual(env, []string{"UPC_ENV=dev", "A=1"}) {
		t.Errorf("active env: Env = %v", env)
	}
}
