package poetry

import (
	"context"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/pkgreport"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// list shows direct dependencies with `poetry show --top-level`.
func (p *Poetry) list(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, inv, err := p.inventory(ctx, req, false)
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "list dependencies", nil), nil
	}
	if pkgs == nil {
		pkgs = []core.Package{}
	}
	summary := pkgreport.Summary(pkgs)
	if summary == "" {
		summary = "no dependencies"
	}
	return inv.Result(summary, "", pkgs), nil
}

// outdated shows direct dependencies with a newer version available.
func (p *Poetry) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, inv, err := p.inventory(ctx, req, true)
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "check for outdated dependencies", nil), nil
	}
	stale, summary := pkgreport.Outdated(pkgs)
	if summary == "" {
		summary = "all dependencies are up to date"
	}
	return inv.Result(summary, "", stale), nil
}

// inventory runs the read-only query shared by list and outdated and parses it.
func (p *Poetry) inventory(ctx context.Context, req core.Request, outdated bool) ([]core.Package, *invoke.Invocation, error) {
	if err := requireProject(req); err != nil {
		return nil, nil, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	args := []string{"show", "--top-level"}
	if outdated {
		args = append(args, "--outdated")
	}
	out, err := inv.Run(false, invoke.Concat(args, req.Args)...)
	if err != nil {
		return nil, inv, err
	}
	if req.DryRun {
		return nil, inv, nil
	}
	return parseShow(out.Stdout, outdated), inv, nil
}

// parseShow reads `poetry show` text: "name version [latest] description" per
// line. A "(!)" after the name marks a package that is not installed.
func parseShow(text string, outdated bool) []core.Package {
	var pkgs []core.Package
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		p := core.Package{Name: fields[0]}
		rest := fields[1:]
		if len(rest) > 0 && rest[0] == "(!)" {
			rest = rest[1:]
		}
		if len(rest) > 0 {
			p.Version = rest[0]
		}
		if outdated && len(rest) > 1 {
			p.Latest = rest[1]
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}
