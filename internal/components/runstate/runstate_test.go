package runstate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
)

func newState(t *testing.T, dir string) (State, *execx.FakeRunner) {
	t.Helper()
	fake := &execx.FakeRunner{}
	return State{Dir: dir, Ctl: fake}, fake
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestStartWritesReadsAndStopRemoves(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	rec, err := s.Start(context.Background(), execx.Cmd{Name: "srv", Dir: s.Dir})
	if err != nil {
		t.Fatal(err)
	}
	if rec.PID != 1000 || rec.LogPath != s.LogFile() {
		t.Fatalf("record = %+v", rec)
	}
	if pid, err := s.Read(); err != nil || pid != 1000 {
		t.Fatalf("Read = %d, %v", pid, err)
	}
	if !s.Running() {
		t.Error("Running = false after Start")
	}
	if want := filepath.Join(s.Dir, ".universal-package-core", "run", "up.pid"); s.PidFile() != want {
		t.Errorf("PidFile = %q, want %q", s.PidFile(), want)
	}
	out, err := s.Stop()
	if err != nil || out != Stopped {
		t.Fatalf("Stop = %v, %v", out, err)
	}
	if exists(s.PidFile()) || s.Running() || fake.Procs[0].Running {
		t.Error("state left behind after Stop")
	}
}

func TestStartRefusesWhileAlive(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.Start(context.Background(), execx.Cmd{Name: "srv"})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}
	if len(fake.Procs) != 1 {
		t.Errorf("a second process was started: %d", len(fake.Procs))
	}
}

func TestStalePidCleanedByStopAndReplacedByStart(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	rec, err := s.Start(context.Background(), execx.Cmd{Name: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	fake.Crash(rec.PID)
	if s.Running() {
		t.Fatal("crashed process reported running")
	}
	out, err := s.Stop()
	if err != nil || out != Stale {
		t.Fatalf("Stop = %v, %v", out, err)
	}
	if exists(s.PidFile()) {
		t.Error("stale pid file not removed")
	}

	// A crash followed by up (no down) replaces the stale record.
	rec, err = s.Start(context.Background(), execx.Cmd{Name: "srv"})
	if err != nil {
		t.Fatal(err)
	}
	fake.Crash(rec.PID)
	if rec, err = s.Start(context.Background(), execx.Cmd{Name: "srv"}); err != nil || rec.PID != 1002 {
		t.Fatalf("restart = %+v, %v", rec, err)
	}
}

func TestStopWithNothingRecordedIsAbsent(t *testing.T) {
	s, _ := newState(t, t.TempDir())
	if out, err := s.Stop(); err != nil || out != Absent {
		t.Fatalf("Stop = %v, %v", out, err)
	}
}

func TestCorruptPidIsCleaned(t *testing.T) {
	s, _ := newState(t, t.TempDir())
	if err := os.MkdirAll(s.RunDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.PidFile(), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := s.Stop(); err != nil || out != Stale {
		t.Fatalf("Stop = %v, %v", out, err)
	}
	if exists(s.PidFile()) {
		t.Error("corrupt pid file kept")
	}
}

func TestAppsKeepSeparateRunDirectories(t *testing.T) {
	root := t.TempDir()
	fake := &execx.FakeRunner{}
	a := State{Dir: filepath.Join(root, "apps", "a"), Ctl: fake}
	b := State{Dir: filepath.Join(root, "apps", "b"), Ctl: fake}
	if _, err := a.Start(context.Background(), execx.Cmd{Name: "srv"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Start(context.Background(), execx.Cmd{Name: "srv"}); err != nil {
		t.Fatal(err)
	}
	if a.RunDir() == b.RunDir() {
		t.Fatal("run directories are shared")
	}
	if _, err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if a.Running() || !b.Running() {
		t.Errorf("stopping a affected b: a=%v b=%v", a.Running(), b.Running())
	}
}

func TestFailedStartLeavesNothing(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	fake.StartErr = errors.New("boom")
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv"}); err == nil {
		t.Fatal("want error")
	}
	if exists(s.PidFile()) || exists(s.LogFile()) {
		t.Error("files left after failed start")
	}
}

func TestPidWriteFailureStopsProcessAndLeavesNothing(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	s.Ctl = blockPid{FakeRunner: fake, path: s.PidFile()}
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv"}); err == nil {
		t.Fatal("want error")
	}
	if len(fake.Procs) != 1 || fake.Procs[0].Running {
		t.Errorf("process not stopped: %+v", fake.Procs)
	}
	if exists(s.LogFile()) {
		t.Error("log left behind")
	}
}

// blockPid creates a directory at the pid path once the process has started,
// so recording the pid fails.
type blockPid struct {
	*execx.FakeRunner
	path string
}

func (b blockPid) Start(ctx context.Context, cmd execx.Cmd, log string) (int, error) {
	pid, err := b.FakeRunner.Start(ctx, cmd, log)
	if err == nil {
		err = os.MkdirAll(b.path, 0o755)
	}
	return pid, err
}

func TestStopFailureKeepsRecord(t *testing.T) {
	s, fake := newState(t, t.TempDir())
	if _, err := s.Start(context.Background(), execx.Cmd{Name: "srv"}); err != nil {
		t.Fatal(err)
	}
	fake.StopErr = errors.New("denied")
	if _, err := s.Stop(); err == nil {
		t.Fatal("want error")
	}
	if !exists(s.PidFile()) {
		t.Error("pid record removed although the process still runs")
	}
}

func TestForAppNeedsAController(t *testing.T) {
	if _, err := ForApp(context.Background(), "d", ""); err == nil {
		t.Error("want error without a runner")
	}
	ctx := execx.WithRunner(context.Background(), &execx.FakeRunner{})
	if _, err := ForApp(ctx, "d", ""); err != nil {
		t.Errorf("ForApp = %v", err)
	}
}
