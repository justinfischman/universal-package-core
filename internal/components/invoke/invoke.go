// Package invoke runs the commands of one capability call and records them for
// the Result. It is the part of a stack's command plumbing that does not depend
// on which tool the stack drives.
package invoke

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Alternatives names the other programs that can stand in for a program that
// is not installed, in the order to try them. It returns nil when there are
// none.
type Alternatives func(name string) []string

// Invocation runs commands for one capability call and records them for the
// Result. Dry-run is honored by the runner; handlers still list the commands.
type Invocation struct {
	ctx      context.Context
	runner   execx.Runner
	alts     Alternatives
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

// WithAlternatives makes the invocation retry a program that is not installed
// with each of its alternatives. Only the command that finally ran is recorded.
func (i *Invocation) WithAlternatives(alts Alternatives) *Invocation {
	i.alts = alts
	return i
}

// Run executes `binary args...` in the project directory. stream copies the
// program's own output to the user as it happens; capture-only is for output
// upc parses. The captured output is returned even when the command fails.
func (i *Invocation) Run(stream bool, args ...string) (execx.Output, error) {
	return i.exec(i.Req.Dir, stream, i.binary, args)
}

// RunAt is Run in another directory (for example a project being created).
func (i *Invocation) RunAt(dir string, stream bool, args ...string) (execx.Output, error) {
	return i.exec(dir, stream, i.binary, args)
}

// RunProgram executes another program (for example the runtime itself) in the
// project directory.
func (i *Invocation) RunProgram(stream bool, name string, args ...string) (execx.Output, error) {
	return i.exec(i.Req.Dir, stream, name, args)
}

func (i *Invocation) exec(dir string, stream bool, name string, args []string) (execx.Output, error) {
	out, ran, err := i.try(dir, stream, name, args)
	var missing *core.ToolMissingError
	if i.alts != nil && errors.As(err, &missing) {
		for _, alt := range i.alts(name) {
			altOut, altRan, altErr := i.try(dir, stream, alt, args)
			if !errors.As(altErr, &missing) {
				out, ran, err = altOut, altRan, altErr
				break
			}
		}
	}
	i.Commands = append(i.Commands, ran)
	return out, err
}

// try runs one program and returns the command line it ran as a string.
func (i *Invocation) try(dir string, stream bool, name string, args []string) (execx.Output, string, error) {
	cmd := execx.Cmd{Name: name, Args: args, Dir: dir, Stream: stream}
	out, err := i.runner.Run(i.ctx, cmd)
	var exit *execx.ExitError
	if err != nil && stream && errors.As(err, &exit) {
		exit.Stderr = "" // already shown to the user as it was produced
	}
	return out, cmd.String(), err
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
