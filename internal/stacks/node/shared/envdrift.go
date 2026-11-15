package shared

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/drift"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// ModulesDir is the project-local install directory.
const ModulesDir = "node_modules"

// DeclaredDependencies lists the names package.json asks for in
// dependencies and devDependencies, sorted.
func DeclaredDependencies(fsys fs.FS) []string {
	data, err := fs.ReadFile(fsys, "package.json")
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	var names []string
	for name := range pkg.Dependencies {
		names = append(names, name)
	}
	for name := range pkg.DevDependencies {
		if _, dup := pkg.Dependencies[name]; !dup {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// installedVersion reads the version of a package installed at the top of
// node_modules.
func installedVersion(fsys fs.FS, name string) (string, bool) {
	data, err := fs.ReadFile(fsys, path.Join(ModulesDir, name, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return "", false
	}
	return pkg.Version, true
}

// EnvDrift compares the versions a lockfile records for the project's
// declared dependencies (name -> acceptable versions; a name the lock does
// not hold is not checked) with what node_modules holds. source names the
// lockfile in messages. It reports nothing when there is no node_modules:
// that is a not-installed project (or a layout without one, such as Plug'n'Play),
// not drift.
func EnvDrift(fsys fs.FS, source string, locked map[string][]string) []core.Finding {
	if info, err := fs.Stat(fsys, ModulesDir); err != nil || !info.IsDir() {
		return nil
	}
	var missing, stale []string
	for _, name := range DeclaredDependencies(fsys) {
		versions := locked[name]
		if len(versions) == 0 {
			continue
		}
		have, ok := installedVersion(fsys, name)
		switch {
		case !ok:
			missing = append(missing, name)
		case !slices.Contains(versions, have):
			stale = append(stale, fmt.Sprintf("%s (%s installed, %s in %s)", name, have, strings.Join(versions, " or "), source))
		}
	}
	return drift.Findings(ModulesDir, source, missing, stale)
}
