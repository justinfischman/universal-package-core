package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// NewRoot builds the command tree. Each phase adds its commands here.
func NewRoot(d Deps) *cobra.Command {
	// Run parent persistent hooks for subcommands that define their own (cobra
	// otherwise runs only the nearest one). The session is built in a hook.
	cobra.EnableTraverseRunHooks = true

	var flags Flags
	root := &cobra.Command{
		Use:           "upc",
		Short:         "Create, install and manage packages and apps across languages",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newSession(d, flags)
			if err != nil {
				return err
			}
			cmd.SetContext(withSession(cmd.Context(), s))
			return nil
		},
	}
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
	})

	pf := root.PersistentFlags()
	pf.BoolVar(&flags.DryRun, "dry-run", false, "show what would run without changing anything")
	pf.BoolVar(&flags.JSON, "json", false, "machine-readable output (logs become JSON too)")
	pf.BoolVarP(&flags.Yes, "yes", "y", false, "answer yes to prompts")
	pf.BoolVarP(&flags.Verbose, "verbose", "v", false, "debug logging")
	pf.StringVar(&flags.Project, "project", "", "project directory (default: discovered from the current directory)")
	pf.BoolVar(&flags.Global, "global", false, "use global scope instead of the project")

	root.AddCommand(newVersionCmd(), newConfigCmd(d.Registries))
	root.AddCommand(newNewCmd(), newInitCmd(), newDoctorCmd(), newEnvCmd())
	root.AddCommand(newCapabilityCmds()...)
	root.AddCommand(newPublishCmd())
	return root
}

// Execute runs the CLI and returns the process exit code. The error is printed
// once, here.
func Execute(ctx context.Context, d Deps, args []string) int {
	root := NewRoot(d)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return core.ExitOK
	}
	// cobra reports unknown subcommands as plain errors; they are usage errors.
	if !errors.Is(err, core.ErrInvalidRequest) && strings.HasPrefix(err.Error(), "unknown command") {
		err = fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
	}
	_, _ = fmt.Fprintf(d.Stderr, "upc: %v\n", err) // stderr is the last resort; nowhere left to report
	return core.ExitCode(err)
}
