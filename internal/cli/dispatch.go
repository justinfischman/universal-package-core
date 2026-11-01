package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// execute runs a capability with this invocation's runner. When the manager's
// tool is missing it offers to install it, then re-runs the original request.
func (s *Session) execute(ctx context.Context, pm core.PackageManager, c core.Capability, req core.Request) (core.Result, error) {
	s.Logger.Info("dispatch", "pm", pm.ID(), "capability", c, "dir", req.Dir, "dry_run", req.DryRun)
	return s.withInstall(ctx, func(ctx context.Context) (core.Result, error) {
		return pm.Execute(ctx, c, req)
	})
}

// withInstall runs fn with this invocation's runner available to modules. If
// fn fails because a tool is missing, it offers to install the tool and then
// runs fn again, so the original command completes.
func (s *Session) withInstall(ctx context.Context, fn func(context.Context) (core.Result, error)) (core.Result, error) {
	runner, err := s.moduleRunner()
	if err != nil {
		return core.Result{}, err
	}
	ctx = execx.WithRunner(ctx, runner)
	res, err := fn(ctx)
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) {
		return res, err
	}
	if err := s.installTool(ctx, missing); err != nil {
		return core.Result{}, err
	}
	s.Logger.Info("re-running after install", "tool", missing.Tool)
	return fn(ctx)
}

// moduleRunner is the runner modules get. When an environment is active every
// command it runs also receives UPC_ENV=<name> (ADR-0006); upc passes the name,
// never any value from the env file.
func (s *Session) moduleRunner() (execx.Runner, error) {
	res, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if name := res.EffectiveEnvironment(); name != "" {
		return execx.WithEnv(s.Runner, envVar+"="+name), nil
	}
	return s.Runner, nil
}

// installTool asks, then installs a missing tool. Declining, or having no way
// to ask, ends in the original tool-missing error (exit code 4).
func (s *Session) installTool(ctx context.Context, missing *core.ToolMissingError) error {
	if len(missing.InstallCmd) == 0 {
		return missing
	}
	question := fmt.Sprintf("%s is not installed. Install it now?", missing.Tool)
	ok, err := s.Prompter.Confirm(question, true)
	if err != nil {
		return fmt.Errorf("%w; install it and re-run, or pass --yes to install it automatically", missing)
	}
	if !ok {
		s.Logger.Info("install declined", "tool", missing.Tool)
		return missing
	}
	cmd := execx.Cmd{Name: missing.InstallCmd[0], Args: missing.InstallCmd[1:], Stream: true}
	s.Logger.Info("installing tool", "tool", missing.Tool, "cmd", cmd.String())
	if _, err := s.Runner.Run(ctx, cmd); err != nil {
		return fmt.Errorf("install %s: %w", missing.Tool, err)
	}
	return nil
}

// print writes a result. The routed commands are shown for dry runs and JSON;
// otherwise the manager's own output (and --verbose logs) already show them.
func (s *Session) print(res core.Result) error {
	if !s.DryRun && !s.JSON {
		res.Commands = nil
	}
	return s.Printer.Print(res)
}
