package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/workspace"
)

// appTarget is the app selection of `up` and `down`: --all or one --app.
type appTarget struct {
	all bool
	app string
}

func (t appTarget) set() bool { return t.all || t.app != "" }

// names is the app names to act on; empty means every app.
func (t appTarget) names() []string {
	if t.app != "" {
		return []string{t.app}
	}
	return nil
}

// validate rejects combinations that name the target twice or pass arguments
// that no single app owns.
func (t appTarget) validate(member string, args []string) error {
	switch {
	case t.all && t.app != "":
		return fmt.Errorf("%w: --all and --app are mutually exclusive", core.ErrInvalidRequest)
	case member != "":
		return fmt.Errorf("%w: --member cannot be combined with --all or --app", core.ErrInvalidRequest)
	case len(args) > 0:
		return fmt.Errorf("%w: arguments cannot be passed to several apps; run `upc up -w <app> -- args` for one", core.ErrInvalidRequest)
	}
	return nil
}

// loadWorkspace builds the Workspace from the merged config of the project.
func loadWorkspace(s *Session) (*workspace.Workspace, error) {
	dir, err := projectDir(s)
	if err != nil {
		return nil, err
	}
	res, err := s.Store.Load()
	if err != nil {
		return nil, err
	}
	if len(res.Apps) == 0 {
		return nil, fmt.Errorf("%w: no [apps.<name>] tables in the config", core.ErrInvalidRequest)
	}
	return workspace.Load(dir, res.Config)
}

// runApps handles `up` and `down` for --all and --app through the Orchestrator.
func runApps(ctx context.Context, s *Session, c core.Capability, t appTarget) error {
	ws, err := loadWorkspace(s)
	if err != nil {
		return err
	}
	o := workspace.Orchestrator{Workspace: ws, Runner: capabilityRunner{s: s}}
	var names []string
	verb := "started"
	if c == core.CapDown {
		names, err = o.Down(ctx, t.names()...)
		verb = "stopped"
	} else {
		names, err = o.Up(ctx, t.names()...)
	}
	if err != nil {
		return err
	}
	return s.print(core.Result{Summary: fmt.Sprintf("%s %s", verb, strings.Join(names, ", ")), Data: names})
}

// capabilityRunner starts and stops an app through the `up` and `down`
// capabilities of the app's own package manager, so every app uses the same
// path as a single project does. Run state is keyed by the app directory and
// the app name.
type capabilityRunner struct{ s *Session }

func (r capabilityRunner) Start(ctx context.Context, app *workspace.App, env []string) error {
	return r.run(ctx, core.CapUp, app, env)
}

func (r capabilityRunner) Stop(ctx context.Context, app *workspace.App) error {
	return r.run(ctx, core.CapDown, app, nil)
}

func (r capabilityRunner) run(ctx context.Context, c core.Capability, app *workspace.App, env []string) error {
	st, err := resolveStackIn(r.s, app.Dir)
	if err != nil {
		return fmt.Errorf("app %q: %w", app.Name, err)
	}
	req := core.Request{Dir: app.Dir, Member: app.Name, Env: env, DryRun: r.s.DryRun, Yes: r.s.Yes}
	_, err = r.s.execute(ctx, st.PM, c, req)
	return err
}

// workspaceFindings are the cross-app doctor findings of the project's apps.
func workspaceFindings(s *Session, dir string) []findingEntry {
	res, err := s.Store.Load()
	if err != nil || len(res.Apps) == 0 {
		return nil
	}
	ws, err := workspace.Load(dir, res.Config)
	if err != nil {
		return []findingEntry{{Severity: core.SeverityWarn, Message: err.Error()}}
	}
	var out []findingEntry
	for _, f := range ws.Findings() {
		out = append(out, findingEntry{Severity: core.SeverityWarn, Message: fmt.Sprintf("%s: %s", f.Kind, f.Message)})
	}
	if _, err := ws.Order(); err != nil {
		out = append(out, findingEntry{Severity: core.SeverityWarn, Message: err.Error()})
	}
	return out
}
