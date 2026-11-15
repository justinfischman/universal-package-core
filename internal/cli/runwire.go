package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// dispatch runs a capability through the manager and applies the ADR-0016
// fallbacks. The manager answers from its manifest; only when it reports that
// nothing was found does a lower-ranked source (run sources for run, up
// providers for up) get a turn. down first honors a provider that started the
// project.
func (s *Session) dispatch(ctx context.Context, pm core.PackageManager, c core.Capability, req core.Request) (core.Result, error) {
	if c == core.CapDown {
		if res, handled, err := s.downProvider(ctx, req); handled {
			return res, err
		}
	}
	res, err := pm.Execute(ctx, c, req)
	switch {
	case err == nil:
		return res, nil
	case c == core.CapRun && errors.Is(err, scripts.ErrNotDeclared):
		if out, handled, rerr := s.runSource(ctx, req); handled {
			return out, rerr
		}
	case c == core.CapUp && errors.Is(err, scripts.ErrNothingToStart):
		if out, handled, uerr := s.upProvider(ctx, req); handled {
			return out, uerr
		}
	}
	return res, err
}

// runSource runs req.Script as a recipe of the first run source that lists it.
func (s *Session) runSource(ctx context.Context, req core.Request) (core.Result, bool, error) {
	for _, src := range s.Registries.RunSources {
		recipes, err := src.Recipes(ctx, req.Dir)
		if err != nil {
			return core.Result{}, true, err
		}
		if !slices.Contains(recipes, req.Script) {
			continue
		}
		if req.DryRun {
			return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would run recipe %q", req.Script)}, true, nil
		}
		if err := src.Run(ctx, req.Dir, req.Script, req.Args); err != nil {
			return core.Result{}, true, err
		}
		return core.Result{Summary: fmt.Sprintf("ran recipe %q", req.Script)}, true, nil
	}
	return core.Result{}, false, nil
}

func (s *Session) upProvider(ctx context.Context, req core.Request) (core.Result, bool, error) {
	target := core.UpTarget{Dir: req.Dir, App: req.Member}
	for _, p := range s.Registries.UpProviders {
		if !p.Detect(req.Dir) {
			continue
		}
		if req.DryRun {
			return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would start with %s", p.Name())}, true, nil
		}
		state, err := runstate.ForApp(ctx, req.Dir, req.Member)
		if err != nil {
			return core.Result{}, true, err
		}
		if err := p.Up(ctx, target); err != nil {
			return core.Result{}, true, err
		}
		if err := state.SetProvider(p.Name()); err != nil {
			return core.Result{}, true, err
		}
		return core.Result{Summary: "started with " + p.Name()}, true, nil
	}
	return core.Result{}, false, nil
}

// downProvider stops the project through the provider recorded by up. It
// reports handled=false when no provider started the project.
func (s *Session) downProvider(ctx context.Context, req core.Request) (core.Result, bool, error) {
	if req.DryRun {
		return core.Result{}, false, nil
	}
	state, err := runstate.ForApp(ctx, req.Dir, req.Member)
	if err != nil {
		return core.Result{}, true, err
	}
	name, err := state.Provider()
	if err != nil || name == "" {
		return core.Result{}, err != nil, err
	}
	for _, p := range s.Registries.UpProviders {
		if p.Name() != name {
			continue
		}
		if err := p.Down(ctx, core.UpTarget{Dir: req.Dir, App: req.Member}); err != nil {
			return core.Result{}, true, err
		}
		if err := state.ClearProvider(); err != nil {
			return core.Result{}, true, err
		}
		return core.Result{Summary: "stopped with " + name}, true, nil
	}
	return core.Result{}, true, fmt.Errorf("%w: up was started by unknown provider %q", core.ErrInvalidRequest, name)
}
