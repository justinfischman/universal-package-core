package uv

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// begin starts an invocation of the uv binary.
func begin(ctx context.Context, req core.Request) (*shared.Invocation, error) {
	return shared.Begin(ctx, req, Binary)
}
