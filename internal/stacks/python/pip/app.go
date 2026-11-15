package pip

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// appPyproject is the project metadata of a new app; pip has no scaffolding of
// its own, so it is written here. The dev and start scripts are appended after.
const appPyproject = `[project]
name = %q
version = "0.1.0"
requires-python = ">=3.9"
dependencies = []
`

// initApp is filled: Dir/Name gets a pyproject.toml, an empty requirements.txt
// and the app's dev and start scripts. The language's templates are rendered on
// top by the app project type. A failure removes the directory again.
func (p *PIP) initApp(ctx context.Context, req core.Request) (core.Result, error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := p.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		files := []struct{ path, data string }{
			{"pyproject.toml", fmt.Sprintf(appPyproject, req.Name)},
			{manifestFile, ""},
		}
		if err := p.writeAll(root, files); err != nil {
			return core.Result{}, err
		}
		if err := shared.AppendAppScripts(p.fsys, root); err != nil {
			_ = p.fsys.RemoveAll(root)
			return core.Result{}, err
		}
	}
	return s.Result("created app "+req.Name, "create app "+req.Name, nil), nil
}
