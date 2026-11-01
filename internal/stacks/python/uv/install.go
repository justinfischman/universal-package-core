package uv

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// install routes to `uv sync` (project), `uv sync --script f` (script) or a
// venv plus `uv pip install -r requirements.txt` (requirements-only).
func (u *UV) install(ctx context.Context, req core.Request) (core.Result, error) {
	if req.Group != "" && strings.HasPrefix(req.Group, "-") {
		return core.Result{}, fmt.Errorf("%w: %q is not a valid group", core.ErrInvalidRequest, req.Group)
	}
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	switch t.mode {
	case modeRequirements:
		if _, err := fs.Stat(shared.DirFS(req.Dir), ".venv"); err != nil {
			if _, err := inv.Run(true, "venv"); err != nil {
				return core.Result{}, err
			}
		}
		_, err = inv.Run(true, shared.Concat([]string{"pip", "install", "-r", "requirements.txt"}, req.Args)...)
	default:
		args := []string{"sync"}
		if req.Group != "" {
			args = append(args, "--group", req.Group)
		}
		_, err = inv.Run(true, shared.Concat(args, t.scriptArgs(), req.Args)...)
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
	if t.mode == modeRequirements {
		args := []string{"pip", "install", "--upgrade"}
		if len(req.Packages) > 0 {
			args = append(args, req.Packages...)
		} else {
			args = append(args, "-r", "requirements.txt")
		}
		if _, err := inv.Run(true, shared.Concat(args, req.Args)...); err != nil {
			return core.Result{}, err
		}
		return inv.Result("dependencies updated", "update dependencies", nil), nil
	}

	lock := shared.Concat([]string{"lock"}, t.scriptArgs())
	if len(req.Packages) == 0 {
		lock = append(lock, "--upgrade")
	}
	for _, p := range req.Packages {
		lock = append(lock, "--upgrade-package", p)
	}
	if _, err := inv.Run(true, shared.Concat(lock, req.Args)...); err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, shared.Concat([]string{"sync"}, t.scriptArgs())...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("dependencies updated", "update dependencies", nil), nil
}
