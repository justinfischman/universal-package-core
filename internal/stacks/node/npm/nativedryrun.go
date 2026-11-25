package npm

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// NativeDryRunArgs implements core.NativeDryRunner: `npm publish --dry-run`.
func (n *NPM) NativeDryRunArgs(c core.Capability) ([]string, bool) {
	return shared.NativeDryRunArgs(c)
}

// NativeMembers implements core.MemberLister from package.json workspaces.
func (n *NPM) NativeMembers(_ context.Context, dir string) ([]string, error) {
	return shared.NativeMembers(dir)
}
