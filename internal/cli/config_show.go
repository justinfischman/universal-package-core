package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newConfigShowCmd() *cobra.Command {
	var origin bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show every setting; with --origin, the layer each value came from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !origin {
				return fmt.Errorf("%w: config show needs --origin (use `config get` for plain values)", core.ErrInvalidRequest)
			}
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			store := s.Store
			if s.Global {
				store.ProjectRoot = ""
			}
			origins, err := store.Origins(nil)
			if err != nil {
				return err
			}
			lines := make([]string, 0, len(origins))
			for _, o := range origins {
				lines = append(lines, formatOrigin(o))
			}
			return s.Printer.Print(core.Result{Summary: strings.Join(lines, "\n"), Data: origins})
		},
	}
	cmd.Flags().BoolVar(&origin, "origin", false, "show the layer each value came from (values from local.toml are masked)")
	return cmd
}

func formatOrigin(o config.Origin) string {
	if o.Layer == "" {
		return fmt.Sprintf("%s = (unset)", o.Key)
	}
	return fmt.Sprintf("%s = %s (%s)", o.Key, o.Value, o.Layer)
}
