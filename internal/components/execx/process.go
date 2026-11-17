package execx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Controller starts a long-lived background process and later asks whether it
// is alive and stops it. Liveness and stopping are platform specific, so they
// live here rather than in callers (Signal(0) is not portable to Windows).
type Controller interface {
	// Start launches cmd detached from the caller, with stdout and stderr
	// appended to logPath, and returns its pid. A dry run returns pid 0.
	Start(ctx context.Context, cmd Cmd, logPath string) (int, error)
	// Alive reports whether a process with pid is running.
	Alive(pid int) bool
	// StartedAt returns an opaque token that identifies the process instance
	// (its start time), or "" when it cannot be read. A recorded token that no
	// longer matches means the pid was reused by another process.
	StartedAt(pid int) string
	// Stop terminates the process (and, where the platform allows, its
	// children) and returns once it has exited or the grace period ended.
	Stop(pid int) error
}

var _ Controller = OS{}

// stopGrace is how long Stop waits for a graceful exit before killing.
const stopGrace = 3 * time.Second

// Start implements Controller.
func (r OS) Start(_ context.Context, cmd Cmd, logPath string) (int, error) {
	log := r.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	log.Info("route", "cmd", cmd.String(), "dir", cmd.Dir, "dry_run", r.DryRun, "background", true)
	if r.DryRun {
		return 0, nil
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open log %s: %w", logPath, err)
	}
	defer func() { _ = logFile.Close() }()

	// Not CommandContext: the process must outlive the invocation.
	c := exec.Command(cmd.Name, cmd.Args...)
	c.Dir = cmd.Dir
	if len(cmd.Env) > 0 {
		c.Env = append(c.Environ(), cmd.Env...)
	}
	c.Stdout, c.Stderr = logFile, logFile
	detach(c)
	if err := c.Start(); err != nil {
		if errorsIsNotFound(err) {
			return 0, &core.ToolMissingError{Tool: cmd.Name}
		}
		return 0, fmt.Errorf("start %s: %w", cmd, err)
	}
	go func() { _ = c.Wait() }() // reap the child so it never lingers as a zombie
	return c.Process.Pid, nil
}

// Alive implements Controller.
func (OS) Alive(pid int) bool { return pid > 0 && alive(pid) }

// StartedAt implements Controller.
func (OS) StartedAt(pid int) string {
	if pid <= 0 {
		return ""
	}
	return startedAt(pid)
}

// Stop implements Controller. It never reports an already-gone process as an
// error.
func (r OS) Stop(pid int) error {
	if !r.Alive(pid) {
		return nil
	}
	if err := terminate(pid); err != nil && r.Alive(pid) {
		return fmt.Errorf("stop pid %d: %w", pid, err)
	}
	deadline := time.Now().Add(stopGrace)
	for r.Alive(pid) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if !r.Alive(pid) {
		return nil
	}
	if err := kill(pid); err != nil && r.Alive(pid) {
		return fmt.Errorf("kill pid %d: %w", pid, err)
	}
	return nil
}

var errNoController = errors.New("internal error: the command runner cannot manage background processes")

// ControllerFor returns the runner as a Controller.
func ControllerFor(r Runner) (Controller, error) {
	c, ok := r.(Controller)
	if !ok {
		return nil, errNoController
	}
	return c, nil
}
