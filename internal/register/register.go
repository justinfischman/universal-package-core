// Package register is the composition root: the only place that imports
// concrete languages, package managers and project types. Adding one is one
// import and one line here (ADR-0002). Nothing registers itself via init().
package register

import "github.com/justinfischman/universal-package-core/internal/core"

// Default builds the registries for the real binary. Phase 1 ships no concrete
// modules; Phase 2 adds Python, uv and the script project type below.
func Default() (core.Registries, error) {
	regs := core.NewRegistries()
	return regs, nil
}
