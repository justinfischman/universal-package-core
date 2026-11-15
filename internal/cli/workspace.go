package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/workspace"
)

// appListing is one app of `workspace list`, in start order.
type appListing struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Port     int      `json:"port,omitempty"`
	Provides []string `json:"provides,omitempty"`
	Consumes []string `json:"consumes,omitempty"`
}

// workspaceListing is the data of `workspace list`.
type workspaceListing struct {
	Members []string     `json:"members"`
	Apps    []appListing `json:"apps"`
}

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Inspect the members and apps of a multi-app workspace",
	}
	cmd.AddCommand(workspaceSubcommands()...)
	return cmd
}

// workspaceSubcommands is the seam for the workspace verbs. `workspace check`
// (contract lock comparison and the contract doctor findings, ADR-0014
// decision 6) is added here once the contract lock store exists.
func workspaceSubcommands() []*cobra.Command {
	return []*cobra.Command{newWorkspaceListCmd()}
}

func newWorkspaceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the workspace members and the apps in start order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			ws, err := loadWorkspace(s)
			if err != nil {
				return err
			}
			listing, err := listWorkspace(ws)
			if err != nil {
				return err
			}
			return s.print(core.Result{Summary: listing.text(), Data: listing})
		},
	}
}

// listWorkspace collects the members and the apps in start order.
func listWorkspace(ws *workspace.Workspace) (workspaceListing, error) {
	order, err := ws.Order()
	if err != nil {
		return workspaceListing{}, err
	}
	l := workspaceListing{Members: append([]string{}, ws.Members...)}
	for _, a := range order {
		l.Apps = append(l.Apps, appListing{
			Name: a.Name, Path: a.Path, Port: a.Port, Provides: a.Provides, Consumes: a.Consumes,
		})
	}
	return l, nil
}

// text renders the listing for people: members, then the apps in start order.
func (l workspaceListing) text() string {
	var b strings.Builder
	b.WriteString("members:")
	if len(l.Members) == 0 {
		b.WriteString(" (none)")
	}
	for _, m := range l.Members {
		b.WriteString("\n  " + m)
	}
	b.WriteString("\napps (start order):")
	for i, a := range l.Apps {
		fmt.Fprintf(&b, "\n  %d. %s  %s", i+1, a.Name, a.Path)
		if a.Port > 0 {
			fmt.Fprintf(&b, "  port %d", a.Port)
		}
		if len(a.Provides) > 0 {
			fmt.Fprintf(&b, "  provides %s", strings.Join(a.Provides, ","))
		}
		if len(a.Consumes) > 0 {
			fmt.Fprintf(&b, "  consumes %s", strings.Join(a.Consumes, ","))
		}
	}
	return b.String()
}
