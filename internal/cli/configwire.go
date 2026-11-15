package cli

import (
	"context"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// configured runs the command upc's own config defines for c (ADR-0016): the
// `scripts.<name>` argv for run, the `up` argv for up. It reports handled=false
// when the config defines nothing for this request, so the manager's own
// sources and the fallbacks of dispatch apply unchanged. A config command is an
// argv executed directly (no shell) in the request directory.
func (s *Session) configured(ctx context.Context, c core.Capability, req core.Request) (core.Result, bool, error) {
	if c != core.CapRun && c != core.CapUp {
		return core.Result{}, false, nil
	}
	cfg, err := s.Store.Load()
	if err != nil {
		return core.Result{}, true, err
	}
	switch c {
	case core.CapRun:
		argv, ok := cfg.Scripts[req.Script]
		if !ok || req.Script == "" {
			return core.Result{}, false, nil
		}
		return s.runConfigured(ctx, req, argv)
	default:
		if len(cfg.Up) == 0 {
			return core.Result{}, false, nil
		}
		return s.upConfigured(ctx, req, cfg.Up)
	}
}

func (s *Session) runConfigured(ctx context.Context, req core.Request, argv []string) (core.Result, bool, error) {
	runner, ok := execx.FromContext(ctx)
	if !ok {
		return core.Result{}, true, fmt.Errorf("internal error: no command runner in context")
	}
	cmd := execx.Cmd{Name: argv[0], Args: append(append([]string{}, argv[1:]...), req.Args...), Dir: req.Dir, Env: req.Env, Stream: true}
	if req.DryRun {
		return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would run config script %q: %s", req.Script, cmd)}, true, nil
	}
	if _, err := runner.Run(ctx, cmd); err != nil {
		return core.Result{}, true, err
	}
	return core.Result{Summary: fmt.Sprintf("ran config script %q", req.Script)}, true, nil
}

func (s *Session) upConfigured(ctx context.Context, req core.Request, argv []string) (core.Result, bool, error) {
	cmd := execx.Cmd{Name: argv[0], Args: append(append([]string{}, argv[1:]...), req.Args...), Dir: req.Dir, Env: req.Env}
	if req.DryRun {
		return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would start with configured up command: %s", cmd)}, true, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir, req.Member)
	if err != nil {
		return core.Result{}, true, err
	}
	rec, err := state.Start(ctx, cmd)
	if err != nil {
		return core.Result{}, true, err
	}
	return core.Result{Summary: fmt.Sprintf("started (pid %d), log %s", rec.PID, rec.LogPath)}, true, nil
}
