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
			req := core.Request{Dir: st.Dir, DryRun: s.DryRun, Yes: s.Yes, Member: f.member}
			ctx := cmd.Context()

			build, err := s.execute(ctx, st.PM, core.CapBuild, req)
			if err != nil {
				return err
			}
			if !s.DryRun {
				ok, err := s.Prompter.Confirm(fmt.Sprintf("Publish %s with %s? This sends the package to a registry.", st.Dir, st.PM.ID()), false)
				if err != nil {
					return err
				}
				if !ok {
					return s.print(core.Result{Summary: "publish cancelled; nothing was published", Commands: build.Commands})
				}
			}
			req.Args = args
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
	return cmd
}
