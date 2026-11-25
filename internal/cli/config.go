package cli

import (
	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func newConfigCmd(regs core.Registries) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and change upc's own settings",
	}
	cmd.AddCommand(newConfigGetCmd(), newConfigShowCmd(), newConfigSetCmd(regs), newConfigValidateCmd(regs))
	return cmd
}
