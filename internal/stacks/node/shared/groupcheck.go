package shared

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ClassicYarnNote is the ADR-0015 limit of yarn 1.x, shown by `doctor`.
const ClassicYarnNote = "yarn classic (1.x) has no subset install: `install` always installs every group"

// GroupFindings checks the dependency groups an environment declares against
// the project's files: each custom group needs its member manifest, and the
// root manifest must list GroupsGlob. classic marks yarn 1.x, whose install is
// always full. Built-in groups (dev, optional, peer) need nothing.
func GroupFindings(root fs.FS, groups []string, classic bool) []core.Finding {
	custom, err := CustomGroups(groups)
	if err != nil {
		return []core.Finding{{Severity: core.SeverityWarn, Message: err.Error()}}
	}
	if len(custom) == 0 {
		return nil
	}
	var out []core.Finding
	for _, g := range custom {
		if !HasMember(root, g) {
			out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
				"dependency group %q has no member %s/package.json; run `upc add <package> --group %s` to create it", g, GroupDir(g), g)})
		}
	}
	globs, err := WorkspacesOf(root)
	if err == nil && !slices.Contains(globs, GroupsGlob) {
		out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
			"package.json workspaces does not list %q, so the manager does not see the dependency groups", GroupsGlob)})
	}
	if classic {
		out = append(out, core.Finding{Severity: core.SeverityInfo, Message: ClassicYarnNote})
	}
	return out
}
