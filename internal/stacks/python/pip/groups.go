package pip

import (
	"fmt"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// DiagnoseGroups checks declared dependency groups against the project's
// files: pip's group model is one requirements file per group
// (requirements-<group>.txt), so each declared group needs its file and a
// valid file-name-safe group name.
func (p *PIP) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
	var out []core.Finding
	for _, g := range groups {
		file, err := requirementsFile(g)
		if err != nil {
			out = append(out, core.Finding{Severity: core.SeverityWarn, Message: err.Error()})
			continue
		}
		if _, err := fs.Stat(fsys, file); err != nil {
			out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
				"dependency group %q has no %s; run `upc add <package> --group %s` to create it", g, file, g)})
		}
	}
	return out
}
