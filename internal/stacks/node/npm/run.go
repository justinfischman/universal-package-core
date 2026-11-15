package npm

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// runScript resolves what to run in a fixed order (ADR-0008, see
// scripts.Resolve): a file in the project directory (run by node), then a
// package.json script (run by `npm run`).
func (n *NPM) runScript(ctx context.Context, req core.Request) (core.Result, error) {
	if err := requireManifest(req); err != nil {
		return core.Result{}, err
	}
	target, err := scripts.Resolve(invoke.DirFS(req.Dir), req.Script, node.ScriptReader{})
	if err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if target.Kind == scripts.KindFile {
		_, err = inv.RunProgram(true, shared.RuntimeBinary, shared.FileArgs(target.Name, req.Args)...)
	} else {
		_, err = inv.Run(true, withPassthrough([]string{"run", target.Name}, req.Args)...)
	}
	if err != nil {
		return core.Result{}, err
	}
	return inv.Result("", "run "+req.Script, nil), nil
}
