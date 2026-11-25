package poetry

import "github.com/justinfischman/universal-package-core/internal/core"

// NativeDryRunArgs implements core.NativeDryRunner: `poetry publish --dry-run`
// builds nothing new and uploads nothing.
func (p *Poetry) NativeDryRunArgs(c core.Capability) ([]string, bool) {
	if c != core.CapPublish {
		return nil, false
	}
	return []string{"--dry-run"}, true
}
