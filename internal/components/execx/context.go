package execx

import (
	"context"
	"errors"
)

type runnerKey struct{}

// WithRunner returns a context carrying the runner. Package managers are built
// once at startup, before flags decide dry-run or log level, so the CLI hands
// each invocation's runner to them through the context.
func WithRunner(ctx context.Context, r Runner) context.Context {
	return context.WithValue(ctx, runnerKey{}, r)
}

// Ambient is a Runner that uses the runner carried by the context of each
// call. Sources built once at startup use it so they run through the
// invocation's runner (dry run, environment injection).
type Ambient struct{}

// Run implements Runner.
func (Ambient) Run(ctx context.Context, cmd Cmd) (Output, error) {
	r, ok := FromContext(ctx)
	if !ok {
		return Output{}, errors.New("internal error: no command runner in context")
	}
	return r.Run(ctx, cmd)
}

// FromContext returns the runner carried by ctx.
func FromContext(ctx context.Context) (Runner, bool) {
	r, ok := ctx.Value(runnerKey{}).(Runner)
	return r, ok && r != nil
}
