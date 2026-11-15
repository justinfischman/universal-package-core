package pip

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var projectFiles = map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n"}

func scriptsProject(table string) map[string]string {
	return map[string]string{"pyproject.toml": projectFiles["pyproject.toml"] + "\n[tool.upc.scripts]\n" + table}
}

// fixture writes a real project directory: run state lives on the real
// filesystem, so these tests read the project through fsx.OS.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newOSPIP(t *testing.T) *PIP {
	t.Helper()
	p, err := newFor(fsx.OS{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func up(t *testing.T, fake *execx.FakeRunner, req core.Request) (core.Result, error) {
	t.Helper()
	return newOSPIP(t).Execute(execx.WithRunner(context.Background(), fake), core.CapUp, req)
}

func down(t *testing.T, fake *execx.FakeRunner, req core.Request) (core.Result, error) {
	t.Helper()
	return newOSPIP(t).Execute(execx.WithRunner(context.Background(), fake), core.CapDown, req)
}

func TestUpStartsConfiguredCommandInBackgroundAndRecordsState(t *testing.T) {
	dir := fixture(t, with(venv, scriptsProject("dev = \"flask run\"\nup = \"python -m http.server 8000\"\n")))
	fake := &execx.FakeRunner{}
	res, err := up(t, fake, core.Request{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Procs) != 1 {
		t.Fatalf("started %d processes", len(fake.Procs))
	}
	p := fake.Procs[0]
	if p.Cmd.String() != ".venv/bin/python -m http.server 8000" || p.Cmd.Dir != dir {
		t.Errorf("cmd = %q in %q (the configured up command wins over dev)", p.Cmd, p.Cmd.Dir)
	}
	if !strings.Contains(res.Summary, "pid 1000") {
		t.Errorf("summary = %q", res.Summary)
	}
	state := runstate.State{Dir: dir, Ctl: fake}
	if pid, err := state.Read(); err != nil || pid != 1000 {
		t.Errorf("recorded pid = %d, %v", pid, err)
	}
	if p.LogPath != filepath.Join(dir, ".universal-package-core", "run", "up.log") {
		t.Errorf("log = %q", p.LogPath)
	}
}

func TestUpFallsBackToDevThenStart(t *testing.T) {
	for name, tc := range map[string]struct {
		table string
		want  string
	}{
		"dev":   {"dev = \"flask run\"\nstart = \"gunicorn app\"\n", "flask run"},
		"start": {"start = \"gunicorn app\"\n", "gunicorn app"},
	} {
		fake := &execx.FakeRunner{}
		if _, err := up(t, fake, core.Request{Dir: fixture(t, scriptsProject(tc.table))}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := fake.Procs[0].Cmd.String(); got != tc.want {
			t.Errorf("%s: cmd = %q, want %q", name, got, tc.want)
		}
	}
}

func TestUpWithoutAScriptFailsAndLeavesNothing(t *testing.T) {
	dir := fixture(t, projectFiles)
	fake := &execx.FakeRunner{}
	if _, err := up(t, fake, core.Request{Dir: dir}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	if len(fake.Procs) != 0 {
		t.Error("a process was started")
	}
	if _, err := os.Stat(filepath.Join(dir, ".universal-package-core")); err == nil {
		t.Error("run state created for a failed up")
	}
}

func TestUpFailedStartLeavesNoState(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{StartErr: &core.ToolMissingError{Tool: "python3"}}
	_, err := up(t, fake, core.Request{Dir: dir})
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want ToolMissingError", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".universal-package-core", "run", "up.pid")); statErr == nil {
		t.Error("pid file left behind")
	}
}

func TestUpTwiceIsRefusedWhileRunning(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{}
	if _, err := up(t, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := up(t, fake, core.Request{Dir: dir}); !errors.Is(err, runstate.ErrAlreadyRunning) {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}
}

func TestUpDryRunStartsNothing(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{}
	res, err := up(t, fake, core.Request{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Procs) != 0 || !reflect.DeepEqual(res.Commands, []string{"flask run"}) {
		t.Errorf("procs = %d, commands = %v", len(fake.Procs), res.Commands)
	}
}

func TestDownStopsTheProcessAndRemovesThePidFile(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{}
	if _, err := up(t, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	res, err := down(t, fake, core.Request{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if fake.Procs[0].Running || !strings.Contains(res.Summary, "stopped") {
		t.Errorf("running = %v, summary = %q", fake.Procs[0].Running, res.Summary)
	}
	if _, err := os.Stat(filepath.Join(dir, ".universal-package-core", "run", "up.pid")); err == nil {
		t.Error("pid file left behind")
	}
}

func TestDownAfterCrashCleansStalePid(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{}
	if _, err := up(t, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	fake.Crash(1000)
	res, err := down(t, fake, core.Request{Dir: dir})
	if err != nil || !strings.Contains(res.Summary, "already exited") {
		t.Fatalf("down = %q, %v", res.Summary, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".universal-package-core", "run", "up.pid")); err == nil {
		t.Error("stale pid file left behind")
	}
}

func TestDownWithNothingRunningIsNotAnError(t *testing.T) {
	res, err := down(t, &execx.FakeRunner{}, core.Request{Dir: fixture(t, projectFiles)})
	if err != nil || res.Summary != "nothing running" {
		t.Errorf("down = %q, %v", res.Summary, err)
	}
}

func TestUpForwardsRequestEnvToTheChildCommand(t *testing.T) {
	dir := fixture(t, scriptsProject("dev = \"flask run\"\n"))
	fake := &execx.FakeRunner{}
	env := []string{"API_URL=http://localhost:8000/v1", "MODE=dev"}
	if _, err := up(t, fake, core.Request{Dir: dir, Env: env}); err != nil {
		t.Fatal(err)
	}
	if got := fake.Procs[0].Cmd.Env; strings.Join(got, "|") != strings.Join(env, "|") {
		t.Errorf("child env = %v, want %v", got, env)
	}
}
