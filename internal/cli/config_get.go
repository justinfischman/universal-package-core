package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show a setting, or all settings, with the scope each came from",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			res, err := loadForScope(s)
			if err != nil {
				return err
			}
			keys := config.Keys
			if len(args) == 1 {
				keys = args
			}
			values := map[string]string{}
			var summary string
			for _, k := range keys {
				v, err := res.Get(k)
				if err != nil {
					return err
				}
				values[k] = v
				shown := v
				if shown == "" {
					shown = "(unset)"
				}
				summary += fmt.Sprintf("%s = %s", k, shown)
				if src, ok := res.Sources[config.CanonicalKey(k)]; ok {
					summary += fmt.Sprintf(" (%s)", src)
				}
				summary += "\n"
			}
			return s.Printer.Print(core.Result{Summary: trimNewline(summary), Data: values})
		},
	}
}

// loadForScope loads the global scope alone with --global, else the merged view.
func loadForScope(s *Session) (config.Resolved, error) {
	if !s.Global {
		return s.Store.Load()
	}
	g, err := s.Store.LoadScope(config.Global)
	if err != nil {
		return config.Resolved{}, err
	}
	res := config.Resolved{Config: g, Sources: map[string]config.Scope{}}
	for _, k := range config.Keys {
		if v, _ := g.Get(k); v != "" {
			res.Sources[k] = config.Global
		}
	}
	return res, nil
}

func trimNewline(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\n' {
		return s[:n-1]
	}
	return s
}
