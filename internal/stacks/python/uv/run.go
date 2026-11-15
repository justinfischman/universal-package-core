package uv

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// runScript resolves what to run in a fixed order (ADR-0008, see
// scripts.Resolve):
//  1. a file in the project directory,
//  2. [tool.upc.scripts] (a command line, run inside the project environment),
//  3. [project.scripts] (an installed entry point).
func (u *UV) runScript(ctx context.Context, req core.Request) (core.Result, error) {
	target, err := scripts.Resolve(invoke.DirFS(req.Dir), req.Script, python.ScriptReader{})
	if err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, runArgs(target, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("", "run "+req.Script, nil), nil
}

func runArgs(t scripts.Target, extra []string) []string {
	switch t.Kind {
	case scripts.KindFile:
		return invoke.Concat([]string{"run", t.Name}, extra)
	case scripts.KindCommand:
		return invoke.Concat([]string{"run", "--"}, t.Words, extra)
	default:
		return invoke.Concat([]string{"run", "--", t.Name}, extra)
	}
}
