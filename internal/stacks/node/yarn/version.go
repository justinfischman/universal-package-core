package yarn

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// isBerry reports whether the project's yarn is berry (2+) rather than classic
// (1.x), from `yarn --version`. It is asked only where the two differ:
// upgrade/up, audit and publish (berry moves them under `yarn npm`), and the
// list/outdated reports (berry has neither command). Output that is not a
// version (a dry run prints nothing) means classic.
func isBerry(ctx context.Context, dir string) (bool, error) {
	r, ok := execx.FromContext(ctx)
	if !ok {
		return false, errors.New("internal error: no command runner in context")
	}
	out, err := r.Run(ctx, execx.Cmd{Name: Binary, Args: []string{"--version"}, Dir: dir})
	if err != nil {
		return false, err
	}
	major, _, _ := strings.Cut(strings.TrimSpace(out.Stdout), ".")
	n, convErr := strconv.Atoi(major)
	berry := convErr == nil && n >= 2
	slog.Debug("yarn flavor", "version", strings.TrimSpace(out.Stdout), "berry", berry)
	return berry, nil
}

// berryUnsupported is the error for reports berry cannot produce through the
// classic JSON format this manager parses.
func berryUnsupported(c core.Capability) error {
	return &core.UnsupportedError{PackageManager: ID, Capability: c,
		Reason: "yarn berry (2+) has no equivalent command; upc supports this only with yarn classic (1.x)"}
}
