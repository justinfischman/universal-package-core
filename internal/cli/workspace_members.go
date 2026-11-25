package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/workspace"
)

// memberReport is the member comparison of `workspace check` (ADR-0014
// decision 3): the config's `members` against the manager's native list.
type memberReport struct {
	// State is ok, mismatch, skipped or error.
	State string `json:"state"`
	// Detail is the difference, the reason it was skipped, or the error text.
	Detail string `json:"detail,omitempty"`
}

func (m memberReport) failed() bool { return m.State == "mismatch" || m.State == "error" }

// compareMembers asks the workspace root's manager for its native members and
// compares them with the configured ones. No configured members, or a manager
// that cannot list them, is skipped (reported, never a failure).
func compareMembers(ctx context.Context, s *Session, ws *workspace.Workspace) memberReport {
	if len(ws.Members) == 0 {
		return memberReport{State: "skipped", Detail: "no workspace members configured"}
	}
	st, err := resolveStackIn(s, ws.Root)
	if err != nil {
		return memberReport{State: "skipped", Detail: fmt.Sprintf("no package manager resolved: %v", err)}
	}
	lister, ok := st.PM.(core.MemberLister)
	if !ok {
		return memberReport{State: "skipped", Detail: st.PM.ID() + " cannot list its workspace members"}
	}
	native, err := lister.NativeMembers(ctx, ws.Root)
	if err != nil {
		return memberReport{State: "error", Detail: err.Error()}
	}
	root := os.DirFS(ws.Root)
	configured, err := workspace.ExpandMembers(root, ws.Members)
	if err != nil {
		return memberReport{State: "error", Detail: fmt.Sprintf("expand configured members: %v", err)}
	}
	expanded, err := workspace.ExpandMembers(root, native)
	if err != nil {
		return memberReport{State: "error", Detail: fmt.Sprintf("expand the manager's members: %v", err)}
	}
	diff := workspace.CompareMembers(configured, expanded)
	if diff.Empty() {
		return memberReport{State: "ok"}
	}
	return memberReport{State: "mismatch", Detail: diff.String()}
}

// text is the one-clause line of the report.
func (m memberReport) text() string {
	line := "members: " + m.State
	if m.Detail != "" {
		line += "\n  " + m.Detail
	}
	return line
}
