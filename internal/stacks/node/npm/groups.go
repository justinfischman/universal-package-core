package npm

import (
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// DiagnoseGroups checks declared dependency groups against the project's
// files (ADR-0015 option B): each custom group needs its workspace member and
// the root manifest must list the groups glob.
func (n *NPM) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
	return shared.GroupFindings(fsys, groups, false)
}
