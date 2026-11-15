package npm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func scriptsProject(scripts string) map[string]string {
	return map[string]string{"package.json": `{"name":"demo","version":"1.0.0","scripts":{` + scripts + `}}`}
}

func runUp(t *testing.T, c core.Capability, fake *execx.FakeRunner, req core.Request) (core.Result, error) {
	t.Helper()
	return newNPM(t).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

func pidFile(dir string) string {
	return filepath.Join(dir, ".universal-package-core", "run", "up.pid")
}

func TestUpStartsConfiguredScriptInBackgroundAndRecordsState(t *testing.T) {
	dir := fixture(t, scriptsProject(`"dev":"vite","up":"node server.js"`))
	fake := &execx.FakeRunner{}
	res, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir, Args: []string{"--port", "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Procs) != 1 {
		t.Fatalf("started %d processes", len(fake.Procs))
	}
	p := fake.Procs[0]
	if p.Cmd.String() != "npm run up -- --port 1" || p.Cmd.Dir != dir {
		t.Errorf("cmd = %q in %q (the up script wins over dev)", p.Cmd, p.Cmd.Dir)
	}
	if !strings.Contains(res.Summary, "pid 1000") {
		t.Errorf("summary = %q", res.Summary)
	}
	state := runstate.State{Dir: dir, Ctl: fake}
	if pid, err := state.Read(); err != nil || pid != 1000 {
		t.Errorf("recorded pid = %d, %v", pid, err)
	}
}

func TestUpFallsBackToDevThenStart(t *testing.T) {
	for name, tc := range map[string]struct{ scripts, want string }{
		"dev":   {`"dev":"vite","start":"node ."`, "npm run dev"},
		"start": {`"start":"node ."`, "npm run start"},
	} {
		fake := &execx.FakeRunner{}
		if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: fixture(t, scriptsProject(tc.scripts))}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := fake.Procs[0].Cmd.String(); got != tc.want {
			t.Errorf("%s: cmd = %q, want %q", name, got, tc.want)
		}
	}
}

func TestUpWithoutAScriptFailsAndLeavesNothing(t *testing.T) {
	dir := fixture(t, scriptsProject(`"build":"tsc"`))
	fake := &execx.FakeRunner{}
	if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir}); !errors.Is(err, core.ErrInvalidRequest) {
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
	dir := fixture(t, scriptsProject(`"dev":"vite"`))
	fake := &execx.FakeRunner{StartErr: &core.ToolMissingError{Tool: "npm"}}
	_, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir})
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want ToolMissingError", err)
	}
	if _, statErr := os.Stat(pidFile(dir)); statErr == nil {
		t.Error("pid file left behind")
	}
}

func TestUpTwiceIsRefusedWhileRunning(t *testing.T) {
	dir := fixture(t, scriptsProject(`"dev":"vite"`))
	fake := &execx.FakeRunner{}
	if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir}); !errors.Is(err, runstate.ErrAlreadyRunning) {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}
}

func TestUpDryRunStartsNothing(t *testing.T) {
	dir := fixture(t, scriptsProject(`"dev":"vite"`))
	fake := &execx.FakeRunner{}
	res, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Procs) != 0 || !reflect.DeepEqual(res.Commands, []string{"npm run dev"}) {
		t.Errorf("procs = %d, commands = %v", len(fake.Procs), res.Commands)
	}
}

func TestDownStopsTheProcessAndRemovesThePidFile(t *testing.T) {
	dir := fixture(t, scriptsProject(`"dev":"vite"`))
	fake := &execx.FakeRunner{}
	if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	res, err := runUp(t, core.CapDown, fake, core.Request{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if fake.Procs[0].Running || !strings.Contains(res.Summary, "stopped") {
		t.Errorf("running = %v, summary = %q", fake.Procs[0].Running, res.Summary)
	}
	if _, err := os.Stat(pidFile(dir)); err == nil {
		t.Error("pid file left behind")
	}
}

func TestDownAfterCrashCleansStalePid(t *testing.T) {
	dir := fixture(t, scriptsProject(`"dev":"vite"`))
	fake := &execx.FakeRunner{}
	if _, err := runUp(t, core.CapUp, fake, core.Request{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	fake.Crash(1000)
	res, err := runUp(t, core.CapDown, fake, core.Request{Dir: dir})
	if err != nil || !strings.Contains(res.Summary, "already exited") {
		t.Fatalf("down = %q, %v", res.Summary, err)
	}
	if _, err := os.Stat(pidFile(dir)); err == nil {
		t.Error("stale pid file left behind")
	}
}

func TestDownWithNothingRunningIsNotAnError(t *testing.T) {
	res, err := runUp(t, core.CapDown, &execx.FakeRunner{}, core.Request{Dir: fixture(t, projectFiles)})
	if err != nil || res.Summary != "nothing running" {
		t.Errorf("down = %q, %v", res.Summary, err)
	}
}
