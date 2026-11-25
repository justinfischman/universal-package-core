package uv

import (
	"fmt"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// DiagnoseGroups checks declared dependency groups against pyproject.toml:
// every group, dev included, is installed with `uv sync --group <name>`, so
// each must be a [dependency-groups] entry. A group that only exists as an
// extra ([project.optional-dependencies]) is reported separately because
// `--group` does not read extras.
func (u *UV) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
	if len(groups) == 0 {
		return nil
	}
	decls, err := python.ReadGroupDecls(fsys)
	if err != nil {
		return []core.Finding{{Severity: core.SeverityWarn, Message: fmt.Sprintf(
			"cannot check the dependency groups of %s: %v", python.PyprojectFilename, err)}}
	}
	var out []core.Finding
	for _, g := range groups {
		name := python.NormalizeName(g)
		switch {
		case decls.DependencyGroups[name], name == "dev" && decls.UvLegacyDev:
		case decls.Optional[name]:
			out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
				"dependency group %q exists only in [project.optional-dependencies], which `uv sync --group` does not read; run `upc add <package> --group %s` to create it in [dependency-groups]", g, g)})
		default:
			out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
				"dependency group %q is not declared in [dependency-groups] of %s; run `upc add <package> --group %s` to create it", g, python.PyprojectFilename, g)})
		}
	}
	return out
}
