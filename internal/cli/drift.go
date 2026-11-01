package cli

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// driftFindings compares the configured managers with the ones the project's
// own files point at, using only each manager's Detect and rank:
//   - several managers match equally strongly (for example two lockfiles);
//   - the configured manager is not the one the files point at;
//   - package_managers does not list a manager the files point at.
func driftFindings(s *Session, dir string, dirFS fs.FS, res config.Resolved) []findingEntry {
	var found []core.PackageManager
	for _, pm := range s.Registries.PackageManagers.List() {
		if pm.Detect(dirFS) {
			found = append(found, pm)
		}
	}
	top := highestRanked(found, dirFS)
	ids := make([]string, len(top))
	for i, pm := range top {
		ids[i] = pm.ID()
	}
	warn := func(format string, a ...any) findingEntry {
		return findingEntry{Severity: core.SeverityWarn, Message: fmt.Sprintf(format, a...)}
	}

	var out []findingEntry
	configured := res.PackageManagers
	if res.PackageManager != "" && !slices.Contains(configured, res.PackageManager) {
		configured = append([]string{res.PackageManager}, configured...)
	}
	switch {
	case len(ids) > 1 && len(res.PackageManagers) == 0:
		out = append(out, warn("%s all match %s equally (more than one manager's files are present); keep one manager's files and pin it with `upc config set pm <id>`",
			strings.Join(ids, ", "), dir))
	case len(ids) > 0 && len(configured) > 0:
		for _, id := range ids {
			if !slices.Contains(configured, id) {
				out = append(out, warn("%s's files are present but config names %s; run `upc config set pm %s` if the project moved to it, or remove its files",
					id, strings.Join(configured, ", "), id))
			}
		}
	}
	return out
}
