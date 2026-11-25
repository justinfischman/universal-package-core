package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// newPublishCmd builds `publish`: build, confirm, publish (ADR-0016). Publishing
// is outward-facing, so it needs an interactive yes or --yes. Global --dry-run
// runs nothing and reports the build and publish commands that would run.
// Credentials are never read or stored: the manager's own login applies.
func newPublishCmd() *cobra.Command {
	var f capFlags
	var native bool
	cmd := &cobra.Command{
		Use:   "publish [args...]",
		Short: "Build the project, then publish it to a registry (asks first)",
		Args:  usageArgs(cobra.ArbitraryArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			st, err := resolveStack(s)
			if err != nil {
				return err
			}
			nativeArgs, err := nativePublishArgs(st.PM, native)
			if err != nil {
				return err
			}
			req := core.Request{Dir: st.Dir, DryRun: s.DryRun, Yes: s.Yes, Member: f.member}
			ctx := cmd.Context()

			build, err := s.execute(ctx, st.PM, core.CapBuild, req)
			if err != nil {
				return err
			}
			if !s.DryRun && !native {
				ok, err := s.Prompter.Confirm(fmt.Sprintf("Publish %s with %s? This sends the package to a registry.", st.Dir, st.PM.ID()), false)
				if err != nil {
					return err
				}
				if !ok {
					return s.print(core.Result{Summary: "publish cancelled; nothing was published", Commands: build.Commands})
				}
			}
			req.Args = append(append([]string{}, args...), nativeArgs...)
			pub, err := s.execute(ctx, st.PM, core.CapPublish, req)
			if err != nil {
				return err
			}
			pub.Commands = append(append([]string{}, build.Commands...), pub.Commands...)
			return s.print(pub)
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVarP(&f.member, "member", "w", "", "workspace member to act on")
	cmd.Flags().BoolVar(&native, "native-dry-run", false, "build, then run the manager's own publish check; sends nothing")
	return cmd
}

// nativePublishArgs is the manager's publish-check arguments when native is
// set, or nil when it is not. A manager without a native check is a usage error.
func nativePublishArgs(pm core.PackageManager, native bool) ([]string, error) {
	if !native {
		return nil, nil
	}
	if d, ok := pm.(core.NativeDryRunner); ok {
		if args, ok := d.NativeDryRunArgs(core.CapPublish); ok {
			return args, nil
		}
	}
	return nil, fmt.Errorf("%w: %s has no native publish dry run; use the global --dry-run to print the plan", core.ErrInvalidRequest, pm.ID())
}
