package uv

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// initProject routes to `uv init --script <file>` for the script project type,
// `uv init --package <name>` for the package type, `uv init --app <name>` for
// the app type (then adds its dev and start scripts) and plain `uv init` when no
// project type is given.
func (u *UV) initProject(ctx context.Context, req core.Request) (core.Result, error) {
	var args []string
	var created string
	switch req.ProjectType {
	case "script":
		file, err := shared.ScriptFileName(req.Script)
		if err != nil {
			return core.Result{}, err
		}
		args, created = []string{"init", "--script", file}, file
	case "package":
		if err := shared.ValidateProjectName(req.Name); err != nil {
			return core.Result{}, err
		}
		args, created = []string{"init", "--package", "--vcs", "none", "--no-workspace", req.Name}, "package "+req.Name
	case "app":
		if err := shared.ValidateProjectName(req.Name); err != nil {
			return core.Result{}, err
		}
		args, created = []string{"init", "--app", "--vcs", "none", "--no-workspace", req.Name}, "app "+req.Name
	case "":
		args = []string{"init"}
	default:
		return core.Result{}, fmt.Errorf("%w: uv cannot scaffold a %q project yet", core.ErrInvalidRequest, req.ProjectType)
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, shared.Concat(args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	if req.ProjectType == "app" && !req.DryRun {
		if err := shared.AppendAppScripts(u.fsys, filepath.Join(req.Dir, req.Name)); err != nil {
			return core.Result{}, err
		}
	}
	return inv.Result("initialized "+orDefault(created, "project"), "initialize "+orDefault(created, "project"), nil), nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// audit routes to `uv audit`.
func (u *UV) audit(ctx context.Context, req core.Request) (core.Result, error) {
	t, err := resolve(req)
	if err != nil {
		return core.Result{}, err
	}
	if t.mode == modeRequirements {
		return core.Result{}, fmt.Errorf("%w: audit needs a pyproject.toml or a script", core.ErrInvalidRequest)
	}
	return u.stream(ctx, req, shared.Concat([]string{"audit"}, t.scriptArgs(), req.Args), "audit dependencies")
}

// build routes to `uv build`.
func (u *UV) build(ctx context.Context, req core.Request) (core.Result, error) {
	return u.stream(ctx, req, shared.Concat([]string{"build"}, req.Args), "build the project")
}

// publish routes to `uv publish`.
func (u *UV) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return u.stream(ctx, req, shared.Concat([]string{"publish"}, req.Args), "publish the project")
}

// stream runs one uv command whose output is for the user, not for upc.
func (u *UV) stream(ctx context.Context, req core.Request, args []string, what string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("done: "+what, what, nil), nil
}

// Diagnose implements core.Diagnoser: problems uv itself would trip over.
func (u *UV) Diagnose(fsys fs.FS) []core.Finding {
	var findings []core.Finding
	if _, err := fs.Stat(fsys, python.PyprojectFilename); err == nil {
		if _, err := fs.Stat(fsys, "uv.lock"); err != nil {
			findings = append(findings, core.Finding{
				Severity: core.SeverityWarn,
				Message:  "uv.lock is missing, so dependencies are not locked",
				Fix:      core.CapInstall,
			})
		} else {
			findings = append(findings, lockDrift(fsys)...)
		}
		return findings
	}
	if _, err := fs.Stat(fsys, "requirements.txt"); err == nil {
		findings = append(findings, core.Finding{
			Severity: core.SeverityInfo,
			Message:  "only requirements.txt found: add, remove and audit need a pyproject.toml",
		})
	}
	return findings
}
