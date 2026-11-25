package yarn

import (
	"context"
	"fmt"
	"slices"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// NativeDryRunArgs implements core.NativeDryRunner: Yarn Berry's
// `yarn npm publish --dry-run`. Yarn classic has no such flag; publish refuses
// it (see rejectClassicDryRun) rather than risk a real publish.
func (y *Yarn) NativeDryRunArgs(c core.Capability) ([]string, bool) {
	return shared.NativeDryRunArgs(c)
}

// NativeMembers implements core.MemberLister from package.json workspaces,
// which both Yarn flavors read.
func (y *Yarn) NativeMembers(_ context.Context, dir string) ([]string, error) {
	return shared.NativeMembers(dir)
}

// rejectClassicDryRun fails when --dry-run is passed to classic `yarn publish`,
// which does not know the flag and would publish for real.
func rejectClassicDryRun(verb, args []string) error {
	if len(verb) == 1 && slices.Contains(args, "--dry-run") {
		return fmt.Errorf("%w: yarn classic has no publish --dry-run; refusing to publish. Use the global --dry-run to print the plan", core.ErrInvalidRequest)
	}
	return nil
}
