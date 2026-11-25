package pnpm

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// initApp creates Dir/Name: a seeded private application manifest (no library
// fields) with the dev, build and start scripts, then the TypeScript toolchain
// as development dependencies. `pnpm init` is not run: the seed is complete and
// init would add a library "main" field to it. The language's templates
// are rendered on top by the app project type. A failure removes the
// directory again.
func (pn *Pnpm) initApp(ctx context.Context, req core.Request) (result core.Result, err error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := pn.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := pn.fsys.MkdirAll(root, 0o755); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", root, err)
		}
		defer func() {
			if err != nil {
				_ = pn.fsys.RemoveAll(root)
			}
		}()
		seedErr := pn.seed(filepath.Join(root, node.PackageFilename), manifest{
			Name: req.Name, Version: "0.1.0", Private: true, Type: "module",
			Scripts: shared.AppScripts,
			Engines: map[string]string{"node": shared.AppEngine},
		})
		if seedErr != nil {
			return core.Result{}, seedErr
		}
	}
	if _, err := inv.RunAt(root, true, append([]string{"add", "-D"}, shared.AppDevDependencies...)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("created app "+req.Name, "create app "+req.Name, nil), nil
}
