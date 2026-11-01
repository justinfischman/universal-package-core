package npm

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// build is filled: the project's `build` script, else the TypeScript compiler
// when the project has a tsconfig.json.
func (n *NPM) build(ctx context.Context, req core.Request) (core.Result, error) {
	fsys := shared.DirFS(req.Dir)
	p, err := readProject(req)
	if err != nil {
		return core.Result{}, err
	}
	if _, ok := p.Scripts["build"]; ok {
		return n.stream(ctx, req, withPassthrough([]string{"run", "build"}, req.Args), "done: build the project", "build the project")
	}
	if node.IsTypeScript(fsys) {
		args := shared.Concat([]string{"exec", "--no", "--", "tsc"}, req.Args)
		return n.stream(ctx, req, args, "done: build the project", "build the project")
	}
	return core.Result{}, &core.UnsupportedError{PackageManager: ID, Capability: core.CapBuild,
		Reason: "package.json has no `build` script and the project has no tsconfig.json; add a `build` script (for example \"build\": \"tsc\")"}
}

// lint is filled: the project's `lint` script, otherwise Unsupported with
// guidance (npm has no linter of its own).
func (n *NPM) lint(ctx context.Context, req core.Request) (core.Result, error) {
	p, err := readProject(req)
	if err != nil {
		return core.Result{}, err
	}
	if _, ok := p.Scripts["lint"]; !ok {
		return core.Result{}, &core.UnsupportedError{PackageManager: ID, Capability: core.CapLint,
			Reason: "package.json has no `lint` script; add a linter (for example upc add --dev eslint) and a `lint` script"}
	}
	return n.stream(ctx, req, withPassthrough([]string{"run", "lint"}, req.Args), "done: lint the project", "lint the project")
}
