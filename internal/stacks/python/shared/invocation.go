package shared

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Invocation runs commands for one capability call and records them for the
// Result. Dry-run is honored by the runner; handlers still list the commands.
type Invocation struct {
	ctx      context.Context
	runner   execx.Runner
	Req      core.Request
	binary   string
	Commands []string
}

// Begin starts an invocation whose default program is binary. The runner comes
// from the context (see execx.WithRunner).
func Begin(ctx context.Context, req core.Request, binary string) (*Invocation, error) {
	r, ok := execx.FromContext(ctx)
	if !ok {
		return nil, errors.New("internal error: no command runner in context")
	}
	return &Invocation{ctx: ctx, runner: r, Req: req, binary: binary}, nil
}

// Run executes `binary args...` in the project directory. stream copies the
// program's own output to the user as it happens; capture-only is for output
// upc parses.
func (i *Invocation) Run(stream bool, args ...string) (execx.Output, error) {
	return i.RunProgram(stream, i.binary, args...)
}

// RunProgram executes another program (for example a virtual environment's
// python) in the project directory.
func (i *Invocation) RunProgram(stream bool, name string, args ...string) (execx.Output, error) {
	cmd := execx.Cmd{Name: name, Args: args, Dir: i.Req.Dir, Stream: stream}
	i.Commands = append(i.Commands, cmd.String())
	out, err := i.runner.Run(i.ctx, cmd)
	if err != nil {
		var exit *execx.ExitError
		if stream && errors.As(err, &exit) {
			exit.Stderr = "" // already shown to the user as it was produced
		}
		return out, err
	}
	return out, nil
}

// Result builds the Result. A dry run reports the plan instead of the outcome.
func (i *Invocation) Result(done, planned string, data any) core.Result {
	if i.Req.DryRun {
		return core.Result{Summary: "dry run, nothing changed: would " + planned, Commands: i.Commands}
	}
	return core.Result{Summary: done, Data: data, Commands: i.Commands}
}

// Concat joins argument groups into one slice.
func Concat(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// DirFS opens a project directory read-only; "" means the working directory.
func DirFS(dir string) fs.FS {
	if dir == "" {
		dir = "."
	}
	return os.DirFS(dir)
}
