package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// capFlags are the per-command flags the capability table can enable.
type capFlags struct {
	dev    bool
	group  string
	script string
}

// Which flags a capability command accepts.
const (
	flagGroup  = 1 << iota // --group and --dev
	flagScript             // --script
)

// capSpec describes one command that routes a capability. The factory below
// builds every such command from a row, so no command repeats the plumbing.
type capSpec struct {
	use   string
	short string
	cap   core.Capability
	args  cobra.PositionalArgs
	flags int
	// passthrough stops flag parsing after the first positional so the rest is
	// handed to the underlying program untouched.
	passthrough bool
	build       func(args []string) core.Request
}

func packages(args []string) core.Request { return core.Request{Packages: args} }

func noArgs([]string) core.Request { return core.Request{} }

// forward hands every argument to the underlying tool untouched.
func forward(args []string) core.Request { return core.Request{Args: args} }

// capabilityTable is the whole routed command surface.
var capabilityTable = []capSpec{
	{use: "add <package>...", short: "Add packages to the project", cap: core.CapAdd, args: cobra.MinimumNArgs(1), flags: flagGroup | flagScript, build: packages},
	{use: "remove <package>...", short: "Remove packages from the project", cap: core.CapRemove, args: cobra.MinimumNArgs(1), flags: flagGroup | flagScript, build: packages},
	{use: "list", short: "List the project's dependencies", cap: core.CapList, args: cobra.NoArgs, flags: flagScript, build: noArgs},
	{use: "install", short: "Install the project's dependencies", cap: core.CapInstall, args: cobra.NoArgs, flags: flagGroup | flagScript, build: noArgs},
	{use: "update [package]...", short: "Upgrade dependencies (all, or the named ones)", cap: core.CapUpdate, args: cobra.ArbitraryArgs, flags: flagScript, build: packages},
	{use: "outdated", short: "Show dependencies with newer versions", cap: core.CapOutdated, args: cobra.NoArgs, flags: flagScript, build: noArgs},
	{use: "build [args...]", short: "Build the project's distributable artifacts", cap: core.CapBuild, args: cobra.ArbitraryArgs, passthrough: true, build: forward},
	{use: "test [args...]", short: "Run the project's tests", cap: core.CapTest, args: cobra.ArbitraryArgs, passthrough: true, build: forward},
	{use: "lint [args...]", short: "Lint the project", cap: core.CapLint, args: cobra.ArbitraryArgs, passthrough: true, build: forward},
	{use: "audit [args...]", short: "Check dependencies for known vulnerabilities", cap: core.CapAudit, args: cobra.ArbitraryArgs, passthrough: true, build: forward},
	{
		use: "run <script> [args...]", short: "Run a file or a named project script", cap: core.CapRun,
		args: cobra.MinimumNArgs(1), passthrough: true,
		build: func(args []string) core.Request { return core.Request{Script: args[0], Args: args[1:]} },
	},
}

func newCapabilityCmds() []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(capabilityTable))
	for _, spec := range capabilityTable {
		cmds = append(cmds, newCapabilityCmd(spec))
	}
	return cmds
}

func newCapabilityCmd(spec capSpec) *cobra.Command {
	var f capFlags
	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Args:  usageArgs(spec.args),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			req := spec.build(args)
			req.Script = firstNonEmpty(req.Script, f.script)
			if f.dev && f.group != "" {
				return fmt.Errorf("%w: --dev and --group are mutually exclusive", core.ErrInvalidRequest)
			}
			req.Group = f.group
			if f.dev {
				req.Group = "dev"
			}
			st, err := resolveStack(s)
			if err != nil {
				return err
			}
			req.Dir, req.DryRun, req.Yes = st.Dir, s.DryRun, s.Yes
			s.Logger.Debug("stack", "pm", st.PM.ID(), "language", st.Language.ID(), "source", st.Source)
			res, err := s.execute(cmd.Context(), st.PM, spec.cap, req)
			if err != nil {
				return err
			}
			return s.print(res)
		},
	}
	if spec.passthrough {
		cmd.Flags().SetInterspersed(false)
	}
	if spec.flags&flagGroup != 0 {
		cmd.Flags().BoolVarP(&f.dev, "dev", "D", false, "use the development group (same as --group dev)")
		cmd.Flags().StringVar(&f.group, "group", "", "dependency group")
	}
	if spec.flags&flagScript != 0 {
		cmd.Flags().StringVar(&f.script, "script", "", "act on this standalone script instead of the project")
	}
	return cmd
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// usageArgs makes argument-count errors usage errors (exit code 2).
func usageArgs(check cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := check(cmd, args); err != nil {
			return fmt.Errorf("%w: %v", core.ErrInvalidRequest, err)
		}
		return nil
	}
}
