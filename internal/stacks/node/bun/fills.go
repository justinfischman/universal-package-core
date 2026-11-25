package bun

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
)

// build is filled: the project's `build` script, else the TypeScript compiler
// (`bun run tsc`, which runs the project's local binary) when the project has a
// tsconfig.json.
func (bn *Bun) build(ctx context.Context, req core.Request) (core.Result, error) {
	p, err := readProject(req)
	if err != nil {
		return core.Result{}, err
	}
	if _, ok := p.Scripts["build"]; ok {
		return bn.stream(ctx, req, invoke.Concat([]string{"run", "build"}, req.Args), "done: build the project", "build the project")
	}
	if node.IsTypeScript(invoke.DirFS(req.Dir)) {
		return bn.stream(ctx, req, invoke.Concat([]string{"run", "tsc"}, req.Args), "done: build the project", "build the project")
	}
	return core.Result{}, &core.UnsupportedError{PackageManager: ID, Capability: core.CapBuild,
		Reason: "package.json has no `build` script and the project has no tsconfig.json; add a `build` script (for example \"build\": \"tsc\")"}
}

// lint is filled: the project's `lint` script, otherwise Unsupported with
// guidance (bun has no linter of its own).
func (bn *Bun) lint(ctx context.Context, req core.Request) (core.Result, error) {
	p, err := readProject(req)
	if err != nil {
		return core.Result{}, err
	}
	if _, ok := p.Scripts["lint"]; !ok {
		return core.Result{}, &core.UnsupportedError{PackageManager: ID, Capability: core.CapLint,
			Reason: "package.json has no `lint` script; add a linter (for example upc add --dev eslint) and a `lint` script"}
	}
	return bn.stream(ctx, req, invoke.Concat([]string{"run", "lint"}, req.Args), "done: lint the project", "lint the project")
}
