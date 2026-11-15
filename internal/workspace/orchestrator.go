package workspace

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Runner starts and stops one app. The CLI implements it through the `up` and
// `down` capabilities; tests use a fake with scripted failures.
type Runner interface {
	// Start starts the app with the already resolved child env.
	Start(ctx context.Context, app *App, env []string) error
	Stop(ctx context.Context, app *App) error
}

// Orchestrator starts a workspace's apps in dependency order and stops them in
// reverse (ADR-0014 decision 5).
type Orchestrator struct {
	Workspace *Workspace
	Runner    Runner
}

// Up starts the named apps (every app when names is empty), consumed apps
// first. If one fails to start, the apps already started in this call are
// stopped in reverse order and the start failure is returned, joined with any
// stop failures. It returns the names started, in order.
func (o Orchestrator) Up(ctx context.Context, names ...string) ([]string, error) {
	apps, err := o.selected(names)
	if err != nil {
		return nil, err
	}
	var started []*App
	for _, a := range apps {
		env, err := o.Workspace.ChildEnv(a)
		if err == nil {
			err = o.Runner.Start(ctx, a, env)
		}
		if err != nil {
			failure := fmt.Errorf("start app %q: %w", a.Name, err)
			return nil, errors.Join(failure, o.rollback(ctx, started))
		}
		started = append(started, a)
	}
	return appNames(started), nil
}

// rollback stops the started apps in reverse order and keeps going past a
// failed stop so one stuck app does not strand the others.
func (o Orchestrator) rollback(ctx context.Context, started []*App) error {
	var errs []error
	for _, a := range slices.Backward(started) {
		if err := o.Runner.Stop(ctx, a); err != nil {
			errs = append(errs, fmt.Errorf("roll back app %q: %w", a.Name, err))
		}
	}
	return errors.Join(errs...)
}

// Down stops the named apps (every app when names is empty) in reverse start
// order, consumers first. A failed stop does not prevent the others from being
// stopped; every failure is returned. It returns the names stopped, in order.
func (o Orchestrator) Down(ctx context.Context, names ...string) ([]string, error) {
	apps, err := o.selected(names)
	if err != nil {
		return nil, err
	}
	var stopped []*App
	var errs []error
	for _, a := range slices.Backward(apps) {
		if err := o.Runner.Stop(ctx, a); err != nil {
			errs = append(errs, fmt.Errorf("stop app %q: %w", a.Name, err))
			continue
		}
		stopped = append(stopped, a)
	}
	return appNames(stopped), errors.Join(errs...)
}

// selected is the start order restricted to names; unknown names are an error.
func (o Orchestrator) selected(names []string) ([]*App, error) {
	for _, n := range names {
		if _, ok := o.Workspace.App(n); !ok {
			return nil, fmt.Errorf("unknown app %q (known: %v)", n, o.Workspace.Names())
		}
	}
	all, err := o.Workspace.Order()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return all, nil
	}
	return slices.DeleteFunc(all, func(a *App) bool { return !slices.Contains(names, a.Name) }), nil
}

func appNames(apps []*App) []string {
	out := make([]string, len(apps))
	for i, a := range apps {
		out[i] = a.Name
	}
	return out
}
