package pip

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// scriptTemplate is the minimal standalone script init creates for the script
// project type: a PEP 723 metadata block and a body.
const scriptTemplate = "# /// script\n# dependencies = []\n# ///\n\nprint(\"Hello from upc\")\n"

// initProject is filled: create .venv and an empty requirements.txt, or, for
// the script project type, a script file with inline metadata. pip itself has
// no init.
func (p *PIP) initProject(ctx context.Context, req core.Request) (core.Result, error) {
	switch req.ProjectType {
	case "script":
		return p.initScript(ctx, req)
	case "package":
		return p.initPackage(ctx, req)
	case "app":
		return p.initApp(ctx, req)
	case "":
	default:
		return core.Result{}, fmt.Errorf("%w: pip cannot scaffold a %q project yet", core.ErrInvalidRequest, req.ProjectType)
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := s.ensureVenv(); err != nil {
		return core.Result{}, err
	}
	if _, err := fs.Stat(s.fsys, manifestFile); err != nil && !req.DryRun {
		if err := p.fsys.WriteFileAtomic(s.path(manifestFile), nil, 0o644); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", manifestFile, err)
		}
	}
	return s.Result("initialized project", "initialize project", nil), nil
}

func (p *PIP) initScript(ctx context.Context, req core.Request) (core.Result, error) {
	file, err := shared.ScriptFileName(req.Script)
	if err != nil {
		return core.Result{}, err
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := fs.Stat(s.fsys, file); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, file)
	}
	if !req.DryRun {
		if err := p.fsys.WriteFileAtomic(s.path(file), []byte(scriptTemplate), 0o644); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", file, err)
		}
	}
	return s.Result("initialized "+file, "initialize "+file, nil), nil
}

// runScript is filled with the shared lookup (ADR-0008): a file runs under the
// environment's python, a [tool.upc.scripts] command and a [project.scripts]
// entry point run from the environment's bin directory.
func (p *PIP) runScript(ctx context.Context, req core.Request) (core.Result, error) {
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	target, err := scripts.Resolve(s.fsys, req.Script, python.ScriptReader{})
	if err != nil {
		return core.Result{}, err
	}
	var name string
	var args []string
	switch target.Kind {
	case scripts.KindFile:
		name, args = shared.Python(s.fsys, p.goos), []string{target.Name}
	case scripts.KindCommand:
		name, args = shared.Program(s.fsys, p.goos, target.Words[0]), target.Words[1:]
	default:
		name = shared.Program(s.fsys, p.goos, target.Name)
	}
	if _, err := s.RunProgram(true, name, shared.Concat(args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return s.Result("", "run "+req.Script, nil), nil
}

// test is filled: pytest, or the project's `test` script.
func (p *PIP) test(ctx context.Context, req core.Request) (core.Result, error) {
	return p.fillTool(ctx, req, core.CapTest, shared.Pytest, "run the tests")
}

// lint is filled: ruff check, or the project's `lint` script.
func (p *PIP) lint(ctx context.Context, req core.Request) (core.Result, error) {
	return p.fillTool(ctx, req, core.CapLint, shared.Ruff, "lint the project")
}

func (p *PIP) fillTool(ctx context.Context, req core.Request, c core.Capability, tool shared.Tool, what string) (core.Result, error) {
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	words, err := tool.Plan(s.fsys, req.Args)
	if err != nil {
		return core.Result{}, shared.AsUnsupported(err, ID, c)
	}
	if _, err := s.RunProgram(true, shared.Program(s.fsys, p.goos, words[0]), words[1:]...); err != nil {
		return core.Result{}, err
	}
	return s.Result("done: "+what, what, nil), nil
}

// build is filled: `python -m build`, offering to install `build` first.
func (p *PIP) build(ctx context.Context, req core.Request) (core.Result, error) {
	return p.module(ctx, req, "build", "build", []string{"build"}, "build the project")
}

// audit is filled: `python -m pip_audit`, offering to install pip-audit first.
func (p *PIP) audit(ctx context.Context, req core.Request) (core.Result, error) {
	return p.module(ctx, req, "pip_audit", "pip-audit", []string{"pip_audit"}, "audit dependencies")
}

// publish is filled: `python -m twine upload dist/*`, offering to install twine.
func (p *PIP) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return p.module(ctx, req, "twine", "twine", []string{"twine", "upload", "dist/*"}, "publish the project")
}

// module runs `python -m <args> [passthrough]` in the environment after
// checking that the module is installed.
func (p *PIP) module(ctx context.Context, req core.Request, module, pkg string, args []string, what string) (core.Result, error) {
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	py, err := s.existingVenv()
	if err != nil {
		return core.Result{}, err
	}
	if err := s.RequireModule(py, module, pkg); err != nil {
		return core.Result{}, err
	}
	if _, err := s.RunProgram(true, py, shared.Concat([]string{"-m"}, args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return s.Result("done: "+what, what, nil), nil
}
