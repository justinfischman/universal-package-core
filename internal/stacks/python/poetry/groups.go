package poetry

import (
	"fmt"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// mainGroup is poetry's implicit group of [tool.poetry.dependencies]; it needs
// no declaration.
const mainGroup = "main"

// DiagnoseGroups checks declared dependency groups against pyproject.toml:
// `poetry install --with <name>` fails for a group with no
// [tool.poetry.group.<name>] table (or [dependency-groups] entry), and "dev"
// is also satisfied by the legacy [tool.poetry.dev-dependencies] table.
func (p *Poetry) DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding {
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
		if g == mainGroup || decls.Poetry[name] || decls.DependencyGroups[name] {
			continue
		}
		out = append(out, core.Finding{Severity: core.SeverityWarn, Message: fmt.Sprintf(
			"dependency group %q is not declared in [tool.poetry.group.%s] of %s; run `upc add <package> --group %s` to create it", g, g, python.PyprojectFilename, g)})
	}
	return out
}
