package uv

import (
	"context"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// vars are the template values for this request in this target.
func (t target) vars(req core.Request) core.Vars {
	return core.Vars{Packages: req.Packages, GroupName: req.Group, Script: t.scriptArgs(), Dir: req.Dir}
}

// run expands a template and runs it with the request's passthrough args.
func run(inv *shared.Invocation, s core.CommandSpec, v core.Vars, extra []string) error {
	argv, err := s.Expand(v)
	if err != nil {
		return err
	}
	_, err = inv.Run(true, shared.Concat(argv, extra)...)
	return err
}

// install routes to `uv sync` (project), `uv sync --script f` (script) or a
// venv plus `uv pip install -r requirements.txt` (requirements-only).
func (u *UV) install(ctx context.Context, req core.Request) (core.Result, error) {
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := spec.Sync.Group.Args(req.Group); err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	v := t.vars(req)
	switch t.mode {
	case modeRequirements:
		if _, err := fs.Stat(shared.DirFS(req.Dir), ".venv"); err != nil {
			if err := run(inv, spec.Venv, v, nil); err != nil {
				return core.Result{}, err
			}
		}
		err = run(inv, spec.PipInstallReqs, v, req.Args)
	default:
		err = run(inv, spec.Sync, v, req.Args)
	}
	if err != nil {
		return core.Result{}, err
	}
	return inv.Result("dependencies installed", "install dependencies", nil), nil
}

// update upgrades the lock (all packages, or the named ones), then syncs.
func (u *UV) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	v := t.vars(req)
	if t.mode == modeRequirements {
		pip := spec.PipUpgradeReqs
		if len(req.Packages) > 0 {
			pip = spec.PipUpgrade
		}
		if err := run(inv, pip, v, req.Args); err != nil {
			return core.Result{}, err
		}
		return inv.Result("dependencies updated", "update dependencies", nil), nil
	}

	lock := spec.LockAll
	if len(req.Packages) > 0 {
		lock = spec.LockPackages
	}
	if err := run(inv, lock, v, req.Args); err != nil {
		return core.Result{}, err
	}
	if err := run(inv, spec.SyncLocked, v, nil); err != nil {
		return core.Result{}, err
	}
	return inv.Result("dependencies updated", "update dependencies", nil), nil
}
