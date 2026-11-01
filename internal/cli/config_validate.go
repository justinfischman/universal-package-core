package cli

import (
	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newConfigValidateCmd(regs core.Registries) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check the settings against the registered languages, managers and project types",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			res, err := loadForScope(s)
			if err != nil {
				return err
			}
			if err := config.Validate(res.Config, regs); err != nil {
				return err
			}
			return s.Printer.Print(core.Result{Summary: "config is valid", Data: map[string]bool{"valid": true}})
		},
	}
}
