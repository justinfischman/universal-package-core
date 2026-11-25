package pnpm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/pkgreport"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// list shows direct dependencies: `pnpm ls --depth=0 --json`.
func (pn *Pnpm) list(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := pn.query(ctx, req, []string{"ls", "--depth=0", "--json"})
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "list dependencies", nil), nil
	}
	pkgs, err := ParseList(out)
	if err != nil {
		return core.Result{}, err
	}
	summary := pkgreport.Summary(pkgs)
	if summary == "" {
		summary = "no dependencies"
	}
	return inv.Result(summary, "", pkgs), nil
}

// outdated shows direct dependencies that have a newer version available:
// `pnpm outdated --format json`.
func (pn *Pnpm) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := pn.query(ctx, req, []string{"outdated", "--format", "json"})
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "check for outdated dependencies", nil), nil
	}
	pkgs, err := ParseOutdated(out)
	if err != nil {
		return core.Result{}, err
	}
	stale, summary := pkgreport.Outdated(pkgs)
	if summary == "" {
		summary = "all dependencies are up to date"
	}
	return inv.Result(summary, "", stale), nil
}

// query runs a read-only pnpm command whose JSON upc parses. pnpm outdated
// exits 1 when it finds outdated packages but still reports them on stdout,
// so a failed command with output is returned for the parser to judge; one
// without output is a real failure.
func (pn *Pnpm) query(ctx context.Context, req core.Request, args []string) (string, *invoke.Invocation, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return "", nil, err
	}
	out, err := inv.Run(false, invoke.Concat(args, req.Args)...)
	var exit *execx.ExitError
	if err != nil && (!errors.As(err, &exit) || strings.TrimSpace(out.Stdout) == "") {
		return "", inv, err
	}
	return out.Stdout, inv, nil
}

// ParseList reads `pnpm ls --depth=0 --json`: an array with one entry per
// project, each holding dependencies, devDependencies and optionalDependencies
// objects. The direct dependencies of every project are merged and sorted by
// name; a declared dependency that is not installed has no version.
func ParseList(text string) ([]core.Package, error) {
	if strings.TrimSpace(text) == "" {
		return []core.Package{}, nil
	}
	type dep struct {
		Version string `json:"version"`
	}
	var projects []struct {
		Dependencies         map[string]dep `json:"dependencies"`
		DevDependencies      map[string]dep `json:"devDependencies"`
		OptionalDependencies map[string]dep `json:"optionalDependencies"`
	}
	if err := json.Unmarshal([]byte(text), &projects); err != nil {
		return nil, fmt.Errorf("parse pnpm ls output: %w", err)
	}
	seen := map[string]bool{}
	pkgs := []core.Package{}
	for _, p := range projects {
		for _, group := range []map[string]dep{p.Dependencies, p.DevDependencies, p.OptionalDependencies} {
			for name, d := range group {
				if !seen[name] {
					seen[name] = true
					pkgs = append(pkgs, core.Package{Name: name, Version: d.Version})
				}
			}
		}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// ParseOutdated reads `pnpm outdated --format json`: an object keyed by
// package name with `current` and `latest`. An empty object or empty output
// means nothing is outdated.
func ParseOutdated(text string) ([]core.Package, error) {
	if strings.TrimSpace(text) == "" {
		return []core.Package{}, nil
	}
	var doc map[string]struct {
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("parse pnpm outdated output: %w", err)
	}
	pkgs := make([]core.Package, 0, len(doc))
	for name, e := range doc {
		pkgs = append(pkgs, core.Package{Name: name, Version: e.Current, Latest: e.Latest})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
