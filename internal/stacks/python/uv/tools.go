package uv

import (
	"context"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// test is filled: uv has no test runner, so it runs pytest (or the project's
// [tool.upc.scripts] test command) inside the project environment.
func (u *UV) test(ctx context.Context, req core.Request) (core.Result, error) {
	return u.fill(ctx, req, core.CapTest, shared.Pytest, "run the tests")
}

// lint is filled the same way with ruff (or the `lint` script).
func (u *UV) lint(ctx context.Context, req core.Request) (core.Result, error) {
	return u.fill(ctx, req, core.CapLint, shared.Ruff, "lint the project")
}

// fill runs `uv run -- <tool command> [args...]`.
func (u *UV) fill(ctx context.Context, req core.Request, c core.Capability, tool shared.Tool, what string) (core.Result, error) {
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	if t.mode != modeProject {
		return core.Result{}, fmt.Errorf("%w: %s needs a pyproject.toml", core.ErrInvalidRequest, c)
	}
	words, err := tool.Plan(shared.DirFS(req.Dir), req.Args)
	if err != nil {
		return core.Result{}, shared.AsUnsupported(err, ID, c)
	}
	return u.stream(ctx, req, shared.Concat([]string{"run", "--"}, words), what)
}
