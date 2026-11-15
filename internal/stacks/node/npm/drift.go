package npm

import (
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// lockDrift compares package-lock.json with node_modules. An unreadable lock
// reports nothing.
func lockDrift(fsys fs.FS) []core.Finding {
	data, err := fs.ReadFile(fsys, lockFile)
	if err != nil {
		return nil
	}
	var lock struct {
		Packages map[string]struct {
			Version string `json:"version"`
		} `json:"packages"`
	}
	if json.Unmarshal(data, &lock) != nil {
		return nil
	}
	locked := map[string][]string{}
	for key, p := range lock.Packages {
		if name, ok := strings.CutPrefix(key, shared.ModulesDir+"/"); ok && p.Version != "" && !strings.Contains(name, shared.ModulesDir+"/") {
			locked[name] = []string{p.Version}
		}
	}
	return shared.EnvDrift(fsys, lockFile, locked)
}
