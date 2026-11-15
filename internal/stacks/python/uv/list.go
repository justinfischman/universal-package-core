package uv

import (
	"context"
	"regexp"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/pkgreport"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// list shows direct dependencies: `uv tree --depth 1` for a project,
// `uv tree --script f --depth 0` for a script, `uv pip list` for requirements.
func (u *UV) list(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, inv, err := u.inventory(ctx, req, false)
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

// outdated shows direct dependencies that have a newer version available.
func (u *UV) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, inv, err := u.inventory(ctx, req, true)
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
func (u *UV) inventory(ctx context.Context, req core.Request, outdated bool) ([]core.Package, *invoke.Invocation, error) {
	t, err := resolve(req)
	if err != nil {
		return nil, nil, err
	}
	inv, err := begin(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	var args []string
	switch t.mode {
	case modeProject:
		args = []string{"tree", "--depth", "1"}
	case modeScript:
		args = []string{"tree", "--script", t.script, "--depth", "0"}
	case modeRequirements:
		args = []string{"pip", "list", "--format", "json"}
		if outdated {
			args = append(args, "--outdated")
		}
	}
	if outdated && t.mode != modeRequirements {
		args = append(args, "--outdated")
	}
	out, err := inv.Run(false, invoke.Concat(args, req.Args)...)
	if err != nil {
		return nil, inv, err
	}
	if req.DryRun {
		return nil, inv, nil
	}
	if t.mode == modeRequirements {
		pkgs, err := shared.ParsePipList(out.Stdout)
		return pkgs, inv, err
	}
	return parseTree(out.Stdout, t.mode == modeProject), inv, nil
}

var latestRe = regexp.MustCompile(`\(latest: v?([^)\s]+)\)`)

// parseTree reads `uv tree` text. In project mode the first line is the
// project and its dependencies are the lines drawn with a tree branch; in
// script mode (depth 0) every line is a dependency.
func parseTree(text string, hasRoot bool) []core.Package {
	var pkgs []core.Package
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \r")
		if line == "" {
			continue
		}
		if hasRoot {
			if i == 0 {
				continue
			}
			rest, ok := cutBranch(line)
			if !ok {
				continue
			}
			line = rest
		}
		if p, ok := parseTreeLine(line); ok {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

func cutBranch(line string) (string, bool) {
	for _, prefix := range []string{"├── ", "└── "} {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return rest, true
		}
	}
	return "", false
}

func parseTreeLine(line string) (core.Package, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return core.Package{}, false
	}
	p := core.Package{Name: fields[0]}
	if len(fields) > 1 && strings.HasPrefix(fields[1], "v") {
		p.Version = strings.TrimPrefix(fields[1], "v")
	}
	if m := latestRe.FindStringSubmatch(line); m != nil {
		p.Latest = m[1]
	}
	return p, true
}
