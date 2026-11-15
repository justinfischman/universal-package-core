package uv

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// add routes to `uv add [--dev|--group g] [--script f] pkg[constraint] [args...]`.
func (u *UV) add(ctx context.Context, req core.Request) (core.Result, error) {
	return u.change(ctx, req, spec.Add, "add", "added")
}

// remove routes to `uv remove`, with the same shape as add.
func (u *UV) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return u.change(ctx, req, spec.Remove, "remove", "removed")
}

// ConstraintDialect makes uv a core.Versioned manager: it renders add's
// Request.Version in PEP 440 syntax.
func (u *UV) ConstraintDialect() core.Dialect { return spec.Dialect }

func (u *UV) change(ctx context.Context, req core.Request, cs core.CommandSpec, verb, past string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	if _, err := cs.Group.Args(req.Group); err != nil {
		return core.Result{}, err
	}
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	if t.mode == modeRequirements {
		return core.Result{}, fmt.Errorf("%w: %s needs a pyproject.toml or a script; this directory only has requirements.txt", core.ErrInvalidRequest, verb)
	}
	v := t.vars(req)
	if v.Constraint, err = req.RenderConstraint(spec.Dialect, ID); err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if err := run(inv, cs, v, req.Args); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	return inv.Result(past+" "+list, verb+" "+list, nil), nil
}
