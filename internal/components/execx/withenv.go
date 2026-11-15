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

var _ Controller = envRunner{}

// Start implements Controller; the environment entries apply here too.
func (e envRunner) Start(ctx context.Context, cmd Cmd, logPath string) (int, error) {
	c, ok := e.Runner.(Controller)
	if !ok {
		return 0, errNoController
	}
	cmd.Env = append(append([]string{}, e.env...), cmd.Env...)
	return c.Start(ctx, cmd, logPath)
}

// Alive implements Controller.
func (e envRunner) Alive(pid int) bool {
	c, ok := e.Runner.(Controller)
	return ok && c.Alive(pid)
}

// StartedAt implements Controller.
func (e envRunner) StartedAt(pid int) string {
	c, ok := e.Runner.(Controller)
	if !ok {
		return ""
	}
	return c.StartedAt(pid)
}

// Stop implements Controller.
func (e envRunner) Stop(pid int) error {
	c, ok := e.Runner.(Controller)
	if !ok {
		return errNoController
	}
	return c.Stop(pid)
}
