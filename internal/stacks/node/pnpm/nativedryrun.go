package pnpm

import (
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// NativeDryRunArgs implements core.NativeDryRunner: `pnpm publish --dry-run`.
func (pn *Pnpm) NativeDryRunArgs(c core.Capability) ([]string, bool) {
	return shared.NativeDryRunArgs(c)
}
