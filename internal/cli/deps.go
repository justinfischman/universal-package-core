// Package cli wires cobra commands to core interfaces. It is language-agnostic:
// it never names a concrete language, package manager or project type.
package cli

import (
	"io"
	"log/slog"
	"os"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/logx"
	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// BuildInfo is injected at link time (see the Makefile ldflags).
type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Deps is everything the CLI needs, injected so tests can fake any part and
// nothing is a global.
type Deps struct {
	Registries core.Registries
	Build      BuildInfo
	FS         fsx.FS

	Stdin  *os.File
	Stdout io.Writer
	Stderr io.Writer

	Getenv  func(string) string
	HomeDir func() (string, error)
	Getwd   func() (string, error)

	NewLogger   func(verbose, json bool, w io.Writer) *slog.Logger
	NewRunner   func(logger *slog.Logger, dryRun bool, stdout, stderr io.Writer) execx.Runner
	NewPrompter func(yes bool, in *os.File, out io.Writer) prompt.Prompter
}

// DefaultDeps returns production dependencies around the given registries.
func DefaultDeps(regs core.Registries, build BuildInfo) Deps {
	return Deps{
		Registries: regs,
		Build:      build,
		FS:         fsx.OS{},
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Getenv:     os.Getenv,
		HomeDir:    os.UserHomeDir,
		Getwd:      os.Getwd,
		NewLogger:  logx.New,
		NewRunner: func(l *slog.Logger, dry bool, out, errw io.Writer) execx.Runner {
			return execx.OS{Logger: l, DryRun: dry, Stdout: out, Stderr: errw, Stdin: os.Stdin}
		},
		NewPrompter: prompt.New,
	}
}
