package shared

import "github.com/justinfischman/universal-package-core/internal/core"

// NativeDryRunArgs is the argument every Node manager adds to its publish
// command to validate the package without sending it: `npm publish --dry-run`,
// `pnpm publish --dry-run`, `bun publish --dry-run` and Yarn Berry's
// `yarn npm publish --dry-run`. Other capabilities have no native check.
func NativeDryRunArgs(c core.Capability) ([]string, bool) {
	if c != core.CapPublish {
		return nil, false
	}
	return []string{"--dry-run"}, true
}
