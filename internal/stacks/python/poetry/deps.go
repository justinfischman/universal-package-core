package poetry

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// begin starts an invocation of the poetry binary.
func begin(ctx context.Context, req core.Request) (*shared.Invocation, error) {
	return shared.Begin(ctx, req, Binary)
}

// requireProject checks that the directory holds a pyproject.toml and that the
// request does not name a standalone script, which poetry cannot manage.
func requireProject(req core.Request) error {
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	if req.Script != "" {
		return fmt.Errorf("%w: poetry does not manage script dependencies; use a pyproject.toml project", core.ErrInvalidRequest)
	}
	if _, err := fs.Stat(shared.DirFS(req.Dir), python.PyprojectFilename); err != nil {
		return fmt.Errorf("%w: no pyproject.toml in %s; run `upc init` or `upc new`", core.ErrInvalidRequest, dir)
	}
	return nil
}

func checkGroup(group string) error {
	if strings.HasPrefix(group, "-") {
		return fmt.Errorf("%w: %q is not a valid group", core.ErrInvalidRequest, group)
	}
	return nil
}

// add routes to `poetry add [--group g] pkgs... [args...]`.
func (p *Poetry) add(ctx context.Context, req core.Request) (core.Result, error) {
	return p.change(ctx, req, "add", "added")
}

// remove routes to `poetry remove`, with the same shape as add.
func (p *Poetry) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return p.change(ctx, req, "remove", "removed")
}

func (p *Poetry) change(ctx context.Context, req core.Request, verb, past string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	if err := checkGroup(req.Group); err != nil {
		return core.Result{}, err
	}
	if err := requireProject(req); err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	var group []string
	if req.Group != "" {
		group = []string{"--group", req.Group}
	}
	if _, err := inv.Run(true, shared.Concat([]string{verb}, group, req.Packages, req.Args)...); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	return inv.Result(past+" "+list, verb+" "+list, nil), nil
}

// install routes to `poetry install [--with g]`.
func (p *Poetry) install(ctx context.Context, req core.Request) (core.Result, error) {
	if err := checkGroup(req.Group); err != nil {
		return core.Result{}, err
	}
	if err := requireProject(req); err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	args := []string{"install"}
	if req.Group != "" {
		args = append(args, "--with", req.Group)
	}
	if _, err := inv.Run(true, shared.Concat(args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("dependencies installed", "install dependencies", nil), nil
}

// update routes to `poetry update [pkgs...]`.
func (p *Poetry) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	if err := requireProject(req); err != nil {
		return core.Result{}, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, shared.Concat([]string{"update"}, req.Packages, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("dependencies updated", "update dependencies", nil), nil
}
