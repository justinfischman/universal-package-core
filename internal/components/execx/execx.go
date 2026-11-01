// Package execx runs external commands behind an interface so every routed
// command is logged once, honors dry-run, and can be faked in tests.
package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Cmd describes one command invocation.
type Cmd struct {
	Name   string
	Args   []string
	Dir    string
	Env    []string // extra KEY=VALUE entries appended to the environment
	Stream bool     // copy output to the runner's writers as it is produced
}

// String renders the command for logs and dry-run output.
func (c Cmd) String() string {
	return strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
}

// Output is what a command produced.
type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ExitError is returned when a command runs and exits non-zero.
type ExitError struct {
	Cmd    Cmd
	Code   int
	Stderr string
}

func (e *ExitError) Error() string {
	msg := fmt.Sprintf("%s exited with code %d", e.Cmd, e.Code)
	if s := strings.TrimSpace(e.Stderr); s != "" {
		msg += ": " + s
	}
	return msg
}

// Runner executes commands.
type Runner interface {
	Run(ctx context.Context, cmd Cmd) (Output, error)
}

// OS runs commands with os/exec.
type OS struct {
	Logger *slog.Logger
	DryRun bool
	Stdout io.Writer // used when Cmd.Stream is set
	Stderr io.Writer
}

// Run implements Runner. In dry-run mode it logs and returns without running.
func (r OS) Run(ctx context.Context, cmd Cmd) (Output, error) {
	log := r.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	log.Info("route", "cmd", cmd.String(), "dir", cmd.Dir, "dry_run", r.DryRun)
	if r.DryRun {
		return Output{}, nil
	}

	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	c.Dir = cmd.Dir
	if len(cmd.Env) > 0 {
		c.Env = append(c.Environ(), cmd.Env...)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout, c.Stderr = &stdout, &stderr
	if cmd.Stream {
		if r.Stdout != nil {
			c.Stdout = io.MultiWriter(&stdout, r.Stdout)
		}
		if r.Stderr != nil {
			c.Stderr = io.MultiWriter(&stderr, r.Stderr)
		}
	}

	err := c.Run()
	out := Output{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return out, nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		log.Error("tool missing", "tool", cmd.Name)
		return out, &core.ToolMissingError{Tool: cmd.Name}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		out.ExitCode = exitErr.ExitCode()
		log.Error("command failed", "cmd", cmd.String(), "code", out.ExitCode)
		return out, &ExitError{Cmd: cmd, Code: out.ExitCode, Stderr: out.Stderr}
	}
	return out, fmt.Errorf("run %s: %w", cmd, err)
}
