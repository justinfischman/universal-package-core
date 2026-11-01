package execx

import "context"

type runnerKey struct{}

// WithRunner returns a context carrying the runner. Package managers are built
// once at startup, before flags decide dry-run or log level, so the CLI hands
// each invocation's runner to them through the context.
func WithRunner(ctx context.Context, r Runner) context.Context {
	return context.WithValue(ctx, runnerKey{}, r)
}

// FromContext returns the runner carried by ctx.
func FromContext(ctx context.Context) (Runner, bool) {
	r, ok := ctx.Value(runnerKey{}).(Runner)
	return r, ok && r != nil
}
