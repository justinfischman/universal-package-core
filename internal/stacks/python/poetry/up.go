package poetry

import (
	"context"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// up starts the project's long-running process in the background (ADR-0016):
// the configured "up" command, else a "dev" or "start" script, run through
// `poetry run`. The pid and log are recorded under the app's run directory.
func (p *Poetry) up(ctx context.Context, req core.Request) (core.Result, error) {
	target, err := scripts.ResolveFirst(invoke.DirFS(req.Dir), scripts.UpNames, python.ScriptReader{})
	if err != nil {
		return core.Result{}, err
	}
	cmd := execx.Cmd{Name: Binary, Args: runArgs(target, req.Args), Dir: req.Dir, Env: req.Env}
	if req.DryRun {
		return core.Result{Summary: "dry run, nothing changed: would start " + cmd.String(), Commands: []string{cmd.String()}}, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir, req.Member)
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
func (p *Poetry) down(ctx context.Context, req core.Request) (core.Result, error) {
	if req.DryRun {
		return core.Result{Summary: "dry run, nothing changed: would stop the process started by up"}, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir, req.Member)
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
