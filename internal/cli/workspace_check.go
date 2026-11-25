package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/workspace"
)

// contractEntry is one app of `workspace check`.
type contractEntry struct {
	App string `json:"app"`
	// State is ok, missing, changed or error.
	State string `json:"state"`
	// Detail is the diff summary of a changed contract or the error text.
	Detail string `json:"detail,omitempty"`
	// Wrote is set when --update wrote (or, in a dry run, would write) the lock.
	Wrote bool `json:"wrote,omitempty"`
}

// contractReport is the data of `workspace check`.
type contractReport struct {
	Apps    []contractEntry `json:"apps"`
	Updated bool            `json:"updated"`
	// Members compares the config's members with the manager's native list.
	Members memberReport `json:"members"`
}

func newWorkspaceCheckCmd() *cobra.Command {
	var update bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Compare each app's contract with its committed lock",
		Long: "Compare the contract of each app with its lock under .universal-package-core/contracts.\n" +
			"The command fails when a lock is missing or the contract changed. Locks are written only\n" +
			"with --update, which accepts the current contracts.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := sessionFrom(cmd)
			if err != nil {
				return err
			}
			ws, err := loadWorkspace(s)
			if err != nil {
				return err
			}
			return runWorkspaceCheck(cmd.Context(), s, ws, update)
		},
	}
	cmd.Flags().BoolVar(&update, "update", false, "write the locks from the current contracts (accept the change)")
	return cmd
}

func runWorkspaceCheck(ctx context.Context, s *Session, ws *workspace.Workspace, update bool) error {
	locks := workspace.NewContractLocks(ws)
	results := locks.CheckAll()
	members := compareMembers(ctx, s, ws)
	if len(results) == 0 {
		if members.failed() {
			return fmt.Errorf("workspace members differ from the manager's: %s", members.Detail)
		}
		return fmt.Errorf("%w: no app declares a contract", core.ErrInvalidRequest)
	}
	rep := contractReport{Updated: update && !s.DryRun, Members: members}
	failed := 0
	for _, r := range results {
		e := entryOf(r)
		if update && (r.Err != nil || r.Status.State != workspace.ContractOK) {
			if s.DryRun {
				e.Wrote = true
			} else if err := locks.Update(r.App); err != nil {
				e.State, e.Detail = "error", err.Error()
			} else {
				e.Wrote = true
			}
		}
		if !e.Wrote && e.State != string(workspace.ContractOK) {
			failed++
		}
		rep.Apps = append(rep.Apps, e)
	}
	if err := s.print(core.Result{Summary: rep.text(update), Data: rep}); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("contract check failed for %d of %d apps", failed, len(rep.Apps))
	}
	if members.failed() {
		return fmt.Errorf("workspace members differ from the manager's: %s", members.Detail)
	}
	return nil
}

func entryOf(r workspace.AppContract) contractEntry {
	if r.Err != nil {
		return contractEntry{App: r.App, State: "error", Detail: r.Err.Error()}
	}
	return contractEntry{App: r.App, State: string(r.Status.State), Detail: r.Status.Summary}
}

// text renders one line per app, with a changed contract's diff summary below.
func (r contractReport) text(update bool) string {
	var b strings.Builder
	for i, e := range r.Apps {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s: %s", e.App, e.State)
		if e.Wrote {
			b.WriteString(" -> lock written")
			if !r.Updated {
				b.WriteString(" (dry run: not written)")
			}
		}
		if e.Detail != "" {
			b.WriteString("\n  " + strings.ReplaceAll(strings.TrimRight(e.Detail, "\n"), "\n", "\n  "))
		}
	}
	b.WriteString("\n" + r.Members.text())
	if !update {
		b.WriteString("\n(locks are never written automatically; pass --update to accept the current contracts)")
	}
	return b.String()
}
