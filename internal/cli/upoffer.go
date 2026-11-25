package cli

import (
	"context"
	"fmt"
	"slices"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// skipUpOffer is the choice that creates nothing.
const skipUpOffer = "none: do not create an up command"

// offerUpCommand runs when `up` found nothing to start. In an interactive
// terminal it offers the run-source recipes of the project as the project's `up`
// command and, when one is chosen, saves `up = ["upc", "run", <recipe>]` to the
// project config and starts it. It reports handled=false (so the original
// "nothing to start" error stands) when the session cannot ask, there is no
// project config to write, no recipe exists, or the user declines. Nothing is
// written in a dry run.
func (s *Session) offerUpCommand(ctx context.Context, req core.Request) (core.Result, bool, error) {
	if req.DryRun || s.Store.ProjectRoot == "" {
		return core.Result{}, false, nil
	}
	var recipes []string
	for _, src := range s.Registries.RunSources {
		names, err := src.Recipes(ctx, req.Dir)
		if err != nil {
			return core.Result{}, true, err
		}
		for _, n := range names {
			if !slices.Contains(recipes, n) {
				recipes = append(recipes, n)
			}
		}
	}
	if len(recipes) == 0 {
		return core.Result{}, false, nil
	}
	// A non-interactive prompter cannot choose: keep the original error.
	choice, err := s.Prompter.Choose("No up command found. Save one of these recipes as the project's up command?", append(recipes, skipUpOffer))
	if err != nil || choice == skipUpOffer || !slices.Contains(recipes, choice) {
		return core.Result{}, false, nil
	}
	argv := []string{"upc", "run", choice}
	if err := s.saveUpCommand(argv); err != nil {
		return core.Result{}, true, err
	}
	return s.upConfigured(ctx, req, argv)
}

// saveUpCommand writes argv as `up` in the project config, validated against
// the merged result like `config set`.
func (s *Session) saveUpCommand(argv []string) error {
	merged, err := s.Store.Load()
	if err != nil {
		return err
	}
	candidate := merged.Config
	candidate.Up = argv
	if err := config.Validate(candidate, s.Registries); err != nil {
		return err
	}
	own, err := s.Store.LoadScope(config.Project)
	if err != nil {
		return err
	}
	own.Up = argv
	if err := s.Store.Save(config.Project, own); err != nil {
		return fmt.Errorf("save up command: %w", err)
	}
	s.Logger.Info("saved up command", "argv", argv)
	return nil
}
