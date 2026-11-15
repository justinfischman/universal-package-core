package poetry

import (
	"io/fs"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

type lockedPackage struct {
	Name     string   `toml:"name"`
	Version  string   `toml:"version"`
	Optional bool     `toml:"optional"`
	Category string   `toml:"category"`
	Groups   []string `toml:"groups"`
	Markers  string   `toml:"markers"`
}

// required reports whether a plain install puts the package in the
// environment on every machine: a main-group package with no optional flag or
// environment marker. Anything else may legitimately be absent.
func (l lockedPackage) required() bool {
	if l.Optional || l.Markers != "" || (l.Category != "" && l.Category != "main") {
		return false
	}
	if len(l.Groups) == 0 {
		return true
	}
	for _, g := range l.Groups {
		if g == "main" {
			return true
		}
	}
	return false
}

// lockDrift compares poetry.lock with the project's .venv. An unreadable lock
// or environment reports nothing; the missing-lock and missing-venv cases have
// their own findings.
func lockDrift(fsys fs.FS) []core.Finding {
	data, err := fs.ReadFile(fsys, lockFile)
	if err != nil || !shared.HasVenv(fsys) {
		return nil
	}
	var lock struct {
		Packages []lockedPackage `toml:"package"`
	}
	if toml.Unmarshal(data, &lock) != nil {
		return nil
	}
	want := map[string]string{}
	for _, p := range lock.Packages {
		if p.required() && p.Name != "" {
			want[python.NormalizeName(p.Name)] = p.Version
		}
	}
	return shared.EnvDrift(fsys, lockFile, want)
}
