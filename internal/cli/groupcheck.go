package cli

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// groupDiagnoser is implemented by managers that can check the dependency
// groups an environment declares against the project's files (ADR-0015).
type groupDiagnoser interface {
	DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding
}

// groupFindings reports group drift for every declared environment that lists
// groups, in name order. A manager that cannot check groups reports nothing.
func groupFindings(pm core.PackageManager, root fs.FS, res config.Resolved) []findingEntry {
	d, ok := pm.(groupDiagnoser)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(res.Environments))
	for name, env := range res.Environments {
		if len(env.Groups) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var out []findingEntry
	for _, name := range names {
		for _, f := range d.DiagnoseGroups(root, res.Environments[name].Groups) {
			out = append(out, findingEntry{Severity: f.Severity, Message: fmt.Sprintf("environment %q: %s", name, f.Message)})
		}
	}
	return out
}
