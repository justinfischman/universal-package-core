package uv

import (
	"io/fs"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

const lockFile = "uv.lock"

type lockedPackage struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
	Source  struct {
		Virtual  string `toml:"virtual"`
		Editable string `toml:"editable"`
	} `toml:"source"`
	Dependencies []struct {
		Name   string `toml:"name"`
		Marker string `toml:"marker"`
	} `toml:"dependencies"`
}

// lockDrift compares uv.lock with the project's .venv. It walks the root
// project's dependencies that carry no environment marker (the ones every
// machine installs), so a platform-specific package absent here is not drift.
// An unreadable lock or environment reports nothing.
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
	byName := map[string][]lockedPackage{}
	var queue []string
	for _, p := range lock.Packages {
		name := python.NormalizeName(p.Name)
		if p.Source.Virtual == "." || p.Source.Editable == "." {
			queue = append(queue, name)
		}
		byName[name] = append(byName[name], p)
	}
	want := map[string]string{}
	for seen := map[string]bool{}; len(queue) > 0; queue = queue[1:] {
		name := queue[0]
		if seen[name] {
			continue
		}
		seen[name] = true
		for _, p := range byName[name] {
			for _, d := range p.Dependencies {
				if d.Marker == "" {
					queue = append(queue, python.NormalizeName(d.Name))
				}
			}
		}
		switch pkgs := byName[name]; {
		case len(pkgs) == 0 || isRoot(pkgs[0]):
		case len(pkgs) == 1:
			want[name] = pkgs[0].Version
		default:
			want[name] = "" // forked resolution: any locked version will do
		}
	}
	return shared.EnvDrift(fsys, lockFile, want)
}

func isRoot(p lockedPackage) bool { return p.Source.Virtual == "." || p.Source.Editable == "." }
