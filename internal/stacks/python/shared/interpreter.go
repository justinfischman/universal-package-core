package shared

import (
	"context"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// systemInterpreters are the names a system Python 3 answers to: python3 on
// most Unix systems, python where only that exists (and in virtual
// environments), and py, the Windows launcher.
var systemInterpreters = []string{"python3", "python", "py"}

// InterpreterAlternatives is the invoke.Alternatives for Python: when a system
// interpreter name is not installed, try the other names.
func InterpreterAlternatives(name string) []string {
	var out []string
	known := false
	for _, n := range systemInterpreters {
		if n == name {
			known = true
			continue
		}
		out = append(out, n)
	}
	if !known {
		return nil
	}
	return out
}

// Begin starts an invocation whose default program is binary. A system
// interpreter that is not installed falls back to the other interpreter names,
// so a machine with only python, or only the py launcher, still works.
func Begin(ctx context.Context, req core.Request, binary string) (*invoke.Invocation, error) {
	i, err := invoke.Begin(ctx, req, binary)
	if err != nil {
		return nil, err
	}
	return i.WithAlternatives(InterpreterAlternatives), nil
}
