package uv

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// groupArgs maps upc's group to uv flags: "dev" is uv's --dev.
func groupArgs(group string) ([]string, error) {
	switch {
	case group == "":
		return nil, nil
	case group == "dev":
		return []string{"--dev"}, nil
	case strings.HasPrefix(group, "-"):
		return nil, fmt.Errorf("%w: %q is not a valid group", core.ErrInvalidRequest, group)
	}
	return []string{"--group", group}, nil
}

// add routes to `uv add [--dev|--group g] [--script f] pkgs... [args...]`.
func (u *UV) add(ctx context.Context, req core.Request) (core.Result, error) {
	return u.change(ctx, req, "add", "added")
}

// remove routes to `uv remove`, with the same shape as add.
func (u *UV) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return u.change(ctx, req, "remove", "removed")
}

func (u *UV) change(ctx context.Context, req core.Request, verb, past string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := groupArgs(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	if t.mode == modeRequirements {
		return core.Result{}, fmt.Errorf("%w: %s needs a pyproject.toml or a script; this directory only has requirements.txt", core.ErrInvalidRequest, verb)
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	args := shared.Concat([]string{verb}, group, t.scriptArgs(), req.Packages, req.Args)
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	return inv.Result(past+" "+list, verb+" "+list, nil), nil
}
