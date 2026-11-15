package npm

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// initApp creates Dir/Name: `npm init`, then a private application manifest
// (no library fields) with the dev, build and start scripts, then the
// TypeScript toolchain as development dependencies. The language's templates
// are rendered on top by the app project type. A failure removes the
// directory again.
func (n *NPM) initApp(ctx context.Context, req core.Request) (result core.Result, err error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := n.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := n.fsys.MkdirAll(root, 0o755); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", root, err)
		}
		defer func() {
			if err != nil {
				_ = n.fsys.RemoveAll(root)
			}
		}()
	}
	set := []string{"pkg", "set", "version=0.1.0", "type=module", "engines.node=" + shared.AppEngine}
	for _, name := range shared.AppScriptNames() {
		set = append(set, "scripts."+name+"="+shared.AppScripts[name])
	}
	steps := [][]string{
		{"init", "--yes"},
		set,
		{"pkg", "set", "private=true", "--json"},
		{"pkg", "delete", "main", "scripts.test"},
		append([]string{"install", "--save-dev"}, shared.AppDevDependencies...),
	}
	for _, args := range steps {
		if _, err := inv.RunAt(root, true, args...); err != nil {
			return core.Result{}, err
		}
	}
	return inv.Result("created app "+req.Name, "create app "+req.Name, nil), nil
}
