package pip

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// session is one capability call: the invocation, the project's read-only view
// and the interpreter to use.
type session struct {
	*shared.Invocation
	p    *PIP
	dir  string
	fsys fs.FS
}

func (p *PIP) begin(ctx context.Context, req core.Request) (*session, error) {
	inv, err := shared.Begin(ctx, req, Binary)
	if err != nil {
		return nil, err
	}
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	return &session{Invocation: inv, p: p, dir: dir, fsys: p.fsys.DirFS(dir)}, nil
}

// ensureVenv creates .venv when missing and returns the environment's python.
func (s *session) ensureVenv() (string, error) {
	if !shared.HasVenv(s.fsys) {
		if _, err := s.RunProgram(true, shared.SystemPython(s.p.goos), "-m", "venv", shared.VenvDir); err != nil {
			return "", err
		}
	}
	return shared.VenvPythonPath(s.p.goos), nil
}

// existingVenv returns the environment's python, or an error when there is no
// .venv to act in.
func (s *session) existingVenv() (string, error) {
	py, ok := shared.VenvProgram(s.fsys, s.p.goos, "python")
	if !ok {
		return "", fmt.Errorf("%w: no %s in %s; run `upc install` to create it", core.ErrInvalidRequest, shared.VenvDir, s.dir)
	}
	return py, nil
}

// pip runs `python -m pip args...` in the environment.
func (s *session) pip(stream bool, py string, args ...string) error {
	_, err := s.RunProgram(stream, py, shared.Concat([]string{"-m", "pip"}, args)...)
	return err
}

func (s *session) path(name string) string { return filepath.Join(s.dir, name) }
