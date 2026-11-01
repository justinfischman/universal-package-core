package pip

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// Files of the package layout pip fills in, because pip has no scaffolding of
// its own. The environment is not created here: a virtual environment cannot
// be moved, and `upc install` creates it in the final location.
const (
	packagePyproject = `[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.build_meta"

[project]
name = %q
version = "0.1.0"
description = ""
readme = "README.md"
requires-python = ">=3.9"
dependencies = []

[tool.setuptools.packages.find]
where = ["src"]

[tool.pytest.ini_options]
pythonpath = ["src"]
testpaths = ["tests"]
`
	packageReadme = "# %s\n\nA Python package.\n"
	packageInit   = "\"\"\"%s.\"\"\"\n"
)

// initPackage is filled: Dir/Name gets a src layout with pyproject.toml
// (setuptools), a README, an importable package and an empty requirements.txt.
func (p *PIP) initPackage(ctx context.Context, req core.Request) (core.Result, error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := p.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	module := shared.ModuleName(req.Name)
	files := []struct {
		path string
		data string
	}{
		{"pyproject.toml", fmt.Sprintf(packagePyproject, req.Name)},
		{"README.md", fmt.Sprintf(packageReadme, req.Name)},
		{filepath.Join("src", module, "__init__.py"), fmt.Sprintf(packageInit, req.Name)},
		{manifestFile, ""},
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := p.writeAll(root, files); err != nil {
			return core.Result{}, err
		}
	}
	return s.Result("created package "+req.Name, "create package "+req.Name, nil), nil
}

// writeAll creates the files under root; a failure removes root again.
func (p *PIP) writeAll(root string, files []struct{ path, data string }) (err error) {
	if err := p.fsys.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}
	defer func() {
		if err != nil {
			_ = p.fsys.RemoveAll(root)
		}
	}()
	for _, f := range files {
		dest := filepath.Join(root, f.path)
		if err := p.fsys.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
		}
		if err := p.fsys.WriteFileAtomic(dest, []byte(f.data), 0o644); err != nil {
			return fmt.Errorf("create %s: %w", f.path, err)
		}
	}
	return nil
}
