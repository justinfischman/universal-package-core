package pnpm

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// workspaceFile is where pnpm reads its workspace packages; it ignores the
// manifest's `workspaces` field.
const workspaceFile = "pnpm-workspace.yaml"

// DiagnoseGroups checks declared dependency groups against the project's
// files (ADR-0015 option B): each custom group needs its workspace member and
// the root manifest must list the groups glob. pnpm also needs the glob in
// pnpm-workspace.yaml, which is reported here too.
func (pn *Pnpm) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
	findings := shared.GroupFindings(fsys, groups, false)
	if len(groups) == 0 {
		return findings
	}
	if data, err := fs.ReadFile(fsys, workspaceFile); err != nil || !strings.Contains(string(data), shared.GroupsGlob) {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  workspaceFile + " does not list " + shared.GroupsGlob + ", so pnpm will not find the dependency groups",
		})
	}
	return findings
}

// ensureWorkspaceFile makes pnpm see the group members: it creates
// pnpm-workspace.yaml with the groups glob, or appends the glob to an existing
// `packages:` list. A file it cannot extend safely is an error that says what
// to add.
func (pn *Pnpm) ensureWorkspaceFile(dir string) error {
	path := filepath.Join(dir, workspaceFile)
	data, err := pn.fsys.ReadFile(path)
	if err != nil {
		if _, statErr := pn.fsys.Stat(path); statErr == nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		return pn.fsys.WriteFileAtomic(path, []byte("packages:\n  - '"+shared.GroupsGlob+"'\n"), 0o644)
	}
	text := string(data)
	if strings.Contains(text, shared.GroupsGlob) {
		return nil
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "packages:" {
			lines = append(lines[:i+1], append([]string{"  - '" + shared.GroupsGlob + "'"}, lines[i+1:]...)...)
			return pn.fsys.WriteFileAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		}
	}
	return fmt.Errorf("%w: %s has no `packages:` list; add `- '%s'` to it", core.ErrInvalidRequest, path, shared.GroupsGlob)
}
