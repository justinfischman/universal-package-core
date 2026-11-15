// Package compose is an up provider backed by a compose file (ADR-0016). Each
// app gets its own compose project name so apps in a workspace stay isolated.
package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Binary and Subcommand are the tool this package routes through execx.
const (
	Binary     = "docker"
	Subcommand = "compose"
)

var fileNames = []string{"docker-compose.yml", "compose.yaml"}

// Provider brings a compose project up and down.
type Provider struct {
	runner execx.Runner
}

var _ core.UpProvider = (*Provider)(nil)

// New returns a Provider that runs commands through r.
func New(r execx.Runner) *Provider { return &Provider{runner: r} }

// File returns the compose file name in dir, or "" when there is none.
func File(dir string) string {
	for _, n := range fileNames {
		if info, err := os.Stat(filepath.Join(dir, n)); err == nil && !info.IsDir() {
			return n
		}
	}
	return ""
}

// Name implements core.UpProvider.
func (p *Provider) Name() string { return "compose" }

// Detect implements core.UpProvider.
func (p *Provider) Detect(dir string) bool { return File(dir) != "" }

// Up implements core.UpProvider (detached, so the call returns).
func (p *Provider) Up(ctx context.Context, t core.UpTarget) error {
	return p.run(ctx, t, "up", "-d")
}

// Down implements core.UpProvider.
func (p *Provider) Down(ctx context.Context, t core.UpTarget) error {
	return p.run(ctx, t, "down")
}

func (p *Provider) run(ctx context.Context, t core.UpTarget, verb ...string) error {
	file := File(t.Dir)
	if file == "" {
		return fmt.Errorf("no compose file in %s", t.Dir)
	}
	argv := append([]string{Subcommand, "-p", ProjectName(t), "-f", file}, verb...)
	_, err := p.runner.Run(ctx, execx.Cmd{Name: Binary, Args: argv, Dir: t.Dir, Stream: true})
	return err
}

// ProjectName is the compose project name for a target: the app name, else the
// directory name, reduced to lowercase letters, digits, '-' and '_'.
func ProjectName(t core.UpTarget) string {
	name := t.App
	if name == "" {
		name = filepath.Base(t.Dir)
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}
