package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newConfigSetCmd(regs core.Registries) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Change a setting after checking it against the registered modules",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			key, value := args[0], args[1]

			scope := config.Project
			if s.Global {
				scope = config.Global
			} else if s.Store.ProjectRoot == "" {
				return fmt.Errorf("%w: no project found; run inside a project, pass --project <dir>, or use --global", core.ErrInvalidRequest)
			}

			// Validate the merged result, so a project value that conflicts with
			// a global one (or vice versa) is refused before anything is written.
			merged, err := s.Store.Load()
			if err != nil {
				return err
			}
			candidate, err := merged.Set(key, value)
			if err != nil {
				return err
			}
			note, err := checkManagerSwitch(s, merged, candidate, key)
			if err != nil {
				return err
			}
			if err := config.Validate(candidate, regs); err != nil {
				return err
			}

			own, err := s.Store.LoadScope(scope)
			if err != nil {
				return err
			}
			own, err = own.Set(key, value)
			if err != nil {
				return err
			}
			if s.DryRun {
				s.Logger.Info("dry-run: config not written", "key", key, "value", value, "scope", scope)
				return s.Printer.Print(core.Result{Summary: fmt.Sprintf("would set %s = %s (%s)", key, value, scope)})
			}
			if err := s.Store.Save(scope, own); err != nil {
				return err
			}
			s.Logger.Info("config set", "key", key, "value", value, "scope", scope)
			summary := fmt.Sprintf("set %s = %s (%s)", key, value, scope)
			if note != "" {
				summary += "\n" + note
			}
			return s.Printer.Print(core.Result{Summary: summary})
		},
	}
}
