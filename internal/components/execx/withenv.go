package execx

import "context"

type envRunner struct {
	Runner
	env []string
}

// WithEnv wraps r so every command also gets the KEY=VALUE entries in env.
// Entries on the command itself come later and therefore win.
func WithEnv(r Runner, env ...string) Runner {
	if len(env) == 0 {
		return r
	}
	return envRunner{Runner: r, env: env}
}

// Run implements Runner.
func (e envRunner) Run(ctx context.Context, cmd Cmd) (Output, error) {
	cmd.Env = append(append([]string{}, e.env...), cmd.Env...)
	return e.Runner.Run(ctx, cmd)
}
