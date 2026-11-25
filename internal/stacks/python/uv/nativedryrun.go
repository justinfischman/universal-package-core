package uv

import "github.com/justinfischman/universal-package-core/internal/core"

// NativeDryRunArgs implements core.NativeDryRunner: `uv publish --dry-run`
// checks the built files and metadata without uploading.
func (u *UV) NativeDryRunArgs(c core.Capability) ([]string, bool) {
	if c != core.CapPublish {
		return nil, false
	}
	return []string{"--dry-run"}, true
}
