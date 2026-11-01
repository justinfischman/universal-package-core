package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the upc version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			b := s.Build
			return s.Printer.Print(core.Result{
				Summary: fmt.Sprintf("upc %s", b.Version),
				Data:    b,
			})
		},
	}
}
