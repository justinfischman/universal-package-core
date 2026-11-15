package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/output"
	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Flags are the persistent global flags.
type Flags struct {
	DryRun  bool
	JSON    bool
	Yes     bool
	Verbose bool
	Project string // explicit project directory
	Global  bool   // operate on global config
}

// Session is the per-invocation state built after flags are parsed.
type Session struct {
	Flags
	Deps
	Logger   *slog.Logger
	Printer  output.Printer
	Prompter prompt.Prompter
	Runner   execx.Runner
	Store    config.Store
	// EnvOverride is the environment chosen by `--env` for this invocation; it
	// wins over the saved selection and is never written to config.
	EnvOverride string
}

type sessionKey struct{}

func newSession(d Deps, f Flags) (*Session, error) {
	if f.Global && f.Project != "" {
		return nil, fmt.Errorf("%w: --global and --project are mutually exclusive", core.ErrInvalidRequest)
	}
	globalDir, err := config.GlobalDir(d.Getenv, d.HomeDir)
	if err != nil {
		return nil, err
	}
	logger := d.NewLogger(f.Verbose, f.JSON, d.Stderr)
	// With --json, stdout carries only the result document, so the tools' own
	// output is shown on stderr instead.
	toolOut := d.Stdout
	if f.JSON {
		toolOut = d.Stderr
	}

	root := f.Project
	if root == "" && !f.Global {
		cwd, err := d.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		root = config.FindRoot(d.FS, cwd, globalDir, d.Registries)
	}
	logger.Debug("session", "project_root", root, "global_dir", globalDir, "dry_run", f.DryRun)

	return &Session{
		Flags:    f,
		Deps:     d,
		Logger:   logger,
		Printer:  output.New(f.JSON, d.Stdout),
		Prompter: d.NewPrompter(f.Yes, d.Stdin, d.Stderr),
		Runner:   d.NewRunner(logger, f.DryRun, toolOut, d.Stderr),
		Store:    config.Store{FS: d.FS, GlobalDir: globalDir, ProjectRoot: root, Registries: d.Registries},
	}, nil
}

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// sessionFrom returns the session the root command attached; commands run only
// after the root's PersistentPreRunE, so a missing session is a wiring bug.
func sessionFrom(cmd *cobra.Command) (*Session, error) {
	s, ok := cmd.Context().Value(sessionKey{}).(*Session)
	if !ok {
		return nil, fmt.Errorf("internal error: no session on command %q", cmd.CommandPath())
	}
	return s, nil
}
