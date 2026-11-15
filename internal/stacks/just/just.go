// Package just is a run source backed by a justfile (ADR-0016). It ranks below
// the manifest sources and is skipped silently when `just` is not installed.
package just

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Binary is the tool this package routes through execx.
const Binary = "just"

var fileNames = []string{"justfile", "Justfile", ".justfile"}

// Source lists and runs justfile recipes.
type Source struct {
	runner execx.Runner
}

var _ core.RunSource = (*Source)(nil)

// New returns a Source that runs commands through r.
func New(r execx.Runner) *Source { return &Source{runner: r} }

// HasFile reports whether dir holds a justfile.
func HasFile(dir string) bool {
	for _, n := range fileNames {
		if info, err := os.Stat(filepath.Join(dir, n)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// Recipes implements core.RunSource. It returns nil without error when there
// is no justfile or `just` is missing.
func (s *Source) Recipes(ctx context.Context, dir string) ([]string, error) {
	if !HasFile(dir) {
		return nil, nil
	}
	out, err := s.runner.Run(ctx, execx.Cmd{Name: Binary, Args: []string{"--summary"}, Dir: dir})
	if err != nil {
		var missing *core.ToolMissingError
		if errors.As(err, &missing) {
			return nil, nil
		}
		return nil, err
	}
	return strings.Fields(out.Stdout), nil
}

// Run implements core.RunSource.
func (s *Source) Run(ctx context.Context, dir, recipe string, args []string) error {
	argv := append([]string{recipe}, args...)
	_, err := s.runner.Run(ctx, execx.Cmd{Name: Binary, Args: argv, Dir: dir, Stream: true})
	return err
}
