package pip

import (
	"context"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// up starts the project's long-running process in the background (ADR-0016):
// the configured "up" command, else a "dev" or "start" script, run through
// the project environment. The pid and log are recorded under the app's run directory.
func (p *PIP) up(ctx context.Context, req core.Request) (core.Result, error) {
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	target, err := scripts.ResolveFirst(p.fsys.DirFS(dir), scripts.UpNames, python.ScriptReader{})
	if err != nil {
		return core.Result{}, err
	}
	name, args := p.upArgv(dir, target)
	cmd := execx.Cmd{Name: name, Args: shared.Concat(args, req.Args), Dir: req.Dir}
	if req.DryRun {
		return core.Result{Summary: "dry run, nothing changed: would start " + cmd.String(), Commands: []string{cmd.String()}}, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir)
	if err != nil {
		return core.Result{}, err
	}
	rec, err := state.Start(ctx, cmd)
	if err != nil {
		return core.Result{}, err
	}
	return core.Result{
		Summary:  fmt.Sprintf("started %q (pid %d), log %s", target.Name, rec.PID, rec.LogPath),
		Commands: []string{cmd.String()},
	}, nil
}

// down stops what up started. Nothing running, or a crashed process, is not an
// error: the stale record is cleaned.
func (p *PIP) down(ctx context.Context, req core.Request) (core.Result, error) {
	if req.DryRun {
		return core.Result{Summary: "dry run, nothing changed: would stop the process started by up"}, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir)
	if err != nil {
		return core.Result{}, err
	}
	out, err := state.Stop()
	if err != nil {
		return core.Result{}, err
	}
	switch out {
	case runstate.Stopped:
		return core.Result{Summary: "stopped the process started by up"}, nil
	case runstate.Stale:
		return core.Result{Summary: "the process had already exited; cleaned its stale record"}, nil
	default:
		return core.Result{Summary: "nothing running"}, nil
	}
}

// upArgv mirrors runScript: a file runs under the environment's python, a
// command or entry point from the environment's bin directory.
func (p *PIP) upArgv(dir string, t scripts.Target) (string, []string) {
	fsys := p.fsys.DirFS(dir)
	switch t.Kind {
	case scripts.KindFile:
		return shared.Python(fsys, p.goos), []string{t.Name}
	case scripts.KindCommand:
		return shared.Program(fsys, p.goos, t.Words[0]), t.Words[1:]
	default:
		return shared.Program(fsys, p.goos, t.Name), nil
	}
}
