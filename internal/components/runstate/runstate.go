// Package runstate is the tool-neutral record of one app's background process
// for up/down (ADR-0016): a pid file and a log under
// <app dir>/.universal-package-core/run/. Each app directory has its own state,
// so apps in one workspace never clash, and there is no shared state file.
// Liveness and stopping go through execx so they are portable.
package runstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
)

// Names inside the run directory.
const (
	DirName = ".universal-package-core"
	pidFile = "up.pid"
	logFile = "up.log"
)

// ErrAlreadyRunning is returned by Start when the app's process is alive.
var ErrAlreadyRunning = errors.New("already running")

// Record is a running (or recorded) process.
type Record struct {
	PID     int
	LogPath string
}

// Outcome is what Stop found.
type Outcome string

// Stop outcomes.
const (
	Stopped Outcome = "stopped" // a live process was stopped
	Stale   Outcome = "stale"   // the process was already gone; state cleaned
	Absent  Outcome = "absent"  // nothing was recorded
)

// State owns the run directory of one app and the controller that acts on it.
type State struct {
	Dir string // the app directory
	Ctl execx.Controller
}

// ForApp builds the State for an app directory using the runner in ctx.
func ForApp(ctx context.Context, dir string) (State, error) {
	r, ok := execx.FromContext(ctx)
	if !ok {
		return State{}, errors.New("internal error: no command runner in context")
	}
	ctl, err := execx.ControllerFor(r)
	if err != nil {
		return State{}, err
	}
	return State{Dir: dir, Ctl: ctl}, nil
}

// RunDir is <app dir>/.universal-package-core/run.
func (s State) RunDir() string { return filepath.Join(s.Dir, DirName, "run") }

// PidFile is the pid record path.
func (s State) PidFile() string { return filepath.Join(s.RunDir(), pidFile) }

// LogFile is the process log path.
func (s State) LogFile() string { return filepath.Join(s.RunDir(), logFile) }

// Read returns the recorded pid. A missing record is fs.ErrNotExist.
func (s State) Read() (int, error) {
	data, err := os.ReadFile(s.PidFile())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("corrupt pid file %s", s.PidFile())
	}
	return pid, nil
}

// Running reports whether a recorded process is alive.
func (s State) Running() bool {
	pid, err := s.Read()
	return err == nil && s.Ctl.Alive(pid)
}

// Start launches cmd in the background and records its pid. A stale record
// (process gone) is replaced. If anything fails the process is stopped and no
// pid or log file is left behind.
func (s State) Start(ctx context.Context, cmd execx.Cmd) (Record, error) {
	if pid, err := s.Read(); err == nil && s.Ctl.Alive(pid) {
		return Record{}, fmt.Errorf("%w (pid %d); run down first", ErrAlreadyRunning, pid)
	}
	if err := s.clean(); err != nil { // stale record and the previous log
		return Record{}, err
	}
	if err := os.MkdirAll(s.RunDir(), 0o755); err != nil {
		return Record{}, fmt.Errorf("create run directory: %w", err)
	}
	pid, err := s.Ctl.Start(ctx, cmd, s.LogFile())
	if err != nil {
		_ = s.clean()
		return Record{}, err
	}
	if pid == 0 { // dry run: nothing started
		return Record{}, nil
	}
	if err := os.WriteFile(s.PidFile(), []byte(strconv.Itoa(pid)), 0o644); err != nil {
		_ = s.Ctl.Stop(pid)
		_ = s.clean()
		return Record{}, fmt.Errorf("record pid: %w", err)
	}
	return Record{PID: pid, LogPath: s.LogFile()}, nil
}

// Stop stops the recorded process and removes the pid record (the log stays,
// so a crash can be diagnosed; the next Start replaces it). When the process
// is already gone (a crash) only the stale record is cleaned. The pid record is
// kept if the process cannot be stopped, so a retry is possible.
func (s State) Stop() (Outcome, error) {
	pid, err := s.Read()
	if errors.Is(err, os.ErrNotExist) {
		return Absent, nil
	}
	if err != nil { // corrupt record: nothing trustworthy to stop
		return Stale, s.removePid()
	}
	out := Stale
	if s.Ctl.Alive(pid) {
		if err := s.Ctl.Stop(pid); err != nil {
			return "", err
		}
		out = Stopped
	}
	return out, s.removePid()
}

func (s State) removePid() error {
	if err := os.Remove(s.PidFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", s.PidFile(), err)
	}
	return nil
}

// clean removes the pid record and the log; missing files are fine.
func (s State) clean() error {
	for _, p := range []string{s.PidFile(), s.LogFile()} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return nil
}
