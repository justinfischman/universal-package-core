package poetry

import (
	"context"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// initProject routes to `poetry new --src <name>` for the package type and to
// `poetry init --no-interaction` otherwise. Poetry has no standalone scripts,
// and other project types are not scaffolded yet.
func (p *Poetry) initProject(ctx context.Context, req core.Request) (core.Result, error) {
	switch req.ProjectType {
	case "package":
		if err := shared.ValidateProjectName(req.Name); err != nil {
			return core.Result{}, err
		}
		return p.stream(ctx, req, shared.Concat([]string{"new", "--src", req.Name}, req.Args), "create package "+req.Name)
	case "":
		return p.stream(ctx, req, shared.Concat([]string{"init", "--no-interaction"}, req.Args), "initialize project")
	}
	return core.Result{}, fmt.Errorf("%w: poetry cannot scaffold a %q project yet", core.ErrInvalidRequest, req.ProjectType)
}

// build routes to `poetry build`.
func (p *Poetry) build(ctx context.Context, req core.Request) (core.Result, error) {
	return p.stream(ctx, req, shared.Concat([]string{"build"}, req.Args), "build the project")
}

// publish routes to `poetry publish`.
func (p *Poetry) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return p.stream(ctx, req, shared.Concat([]string{"publish"}, req.Args), "publish the project")
}

// stream runs one poetry command whose output is for the user, not for upc.
func (p *Poetry) stream(ctx context.Context, req core.Request, args []string, what string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("done: "+what, what, nil), nil
}

// runScript resolves what to run in a fixed order (ADR-0008, see
// scripts.Resolve): a file, [tool.upc.scripts], then [project.scripts]; all run
// inside the project environment through `poetry run`.
func (p *Poetry) runScript(ctx context.Context, req core.Request) (core.Result, error) {
	target, err := scripts.Resolve(shared.DirFS(req.Dir), req.Script, python.ScriptReader{})
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
		return shared.Concat([]string{"run", "python", t.Name}, extra)
	case scripts.KindCommand:
		return shared.Concat([]string{"run"}, t.Words, extra)
	default:
		return shared.Concat([]string{"run", t.Name}, extra)
	}
}

// pipAudit is the tool behind the audit fill: pip-audit, when the project
// declares it or defines an `audit` script.
var pipAudit = shared.Tool{
	Name: "pip-audit", Script: "audit", Command: []string{"pip-audit"}, Dependency: "pip-audit",
	Enable: "add pip-audit as a development dependency (upc add --dev pip-audit) or add an `audit` entry to [tool.upc.scripts] in pyproject.toml",
}

// test is filled: pytest (or the `test` script) inside the project environment.
func (p *Poetry) test(ctx context.Context, req core.Request) (core.Result, error) {
	return p.fill(ctx, req, core.CapTest, shared.Pytest, "run the tests")
}

// lint is filled the same way with ruff (or the `lint` script).
func (p *Poetry) lint(ctx context.Context, req core.Request) (core.Result, error) {
	return p.fill(ctx, req, core.CapLint, shared.Ruff, "lint the project")
}

// audit is filled the same way with pip-audit (or the `audit` script).
func (p *Poetry) audit(ctx context.Context, req core.Request) (core.Result, error) {
	return p.fill(ctx, req, core.CapAudit, pipAudit, "audit dependencies")
}

// fill runs `poetry run <tool command> [args...]`.
func (p *Poetry) fill(ctx context.Context, req core.Request, c core.Capability, tool shared.Tool, what string) (core.Result, error) {
	if err := requireProject(req); err != nil {
		return core.Result{}, err
	}
	words, err := tool.Plan(shared.DirFS(req.Dir), req.Args)
	if err != nil {
		return core.Result{}, shared.AsUnsupported(err, ID, c)
	}
	return p.stream(ctx, req, shared.Concat([]string{"run"}, words), what)
}
