package bun

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/pkgreport"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// list shows direct dependencies: `bun pm ls`. bun has no JSON report, so the
// tree it prints is parsed.
func (bn *Bun) list(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := bn.query(ctx, req, []string{"pm", "ls"})
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "list dependencies", nil), nil
	}
	pkgs := ParseList(out)
	summary := pkgreport.Summary(pkgs)
	if summary == "" {
		summary = "no dependencies"
	}
	return inv.Result(summary, "", pkgs), nil
}

// outdated shows direct dependencies that have a newer version available:
// `bun outdated`, whose table is parsed.
func (bn *Bun) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := bn.query(ctx, req, []string{"outdated"})
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "check for outdated dependencies", nil), nil
	}
	stale, summary := pkgreport.Outdated(ParseOutdated(out))
	if summary == "" {
		summary = "all dependencies are up to date"
	}
	return inv.Result(summary, "", stale), nil
}

// query runs a read-only bun command whose text upc parses. A failed command
// that still printed on stdout is returned for the parser to judge; one
// without output (bun reports a missing lock file on stderr) is a real failure.
func (bn *Bun) query(ctx context.Context, req core.Request, args []string) (string, *invoke.Invocation, error) {
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

// ParseList reads `bun pm ls`: a header line, then one tree line per direct
// dependency (`├── name@version`), sorted by name. Nested lines (indented under
// a `│`) are transitive dependencies and are skipped.
func ParseList(text string) []core.Package {
	pkgs := []core.Package{}
	for _, line := range strings.Split(text, "\n") {
		spec, ok := strings.CutPrefix(strings.TrimRight(line, "\r "), "├── ")
		if !ok {
			if spec, ok = strings.CutPrefix(strings.TrimRight(line, "\r "), "└── "); !ok {
				continue
			}
		}
		spec, _, _ = strings.Cut(spec, " ")
		if i := strings.LastIndex(spec, "@"); i > 0 {
			pkgs = append(pkgs, core.Package{Name: spec[:i], Version: spec[i+1:]})
		}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs
}

// ParseOutdated reads the table `bun outdated` prints: rows of
// `| name (dev) | current | update | latest |` between border and header
// rows. Output without rows means nothing is outdated.
func ParseOutdated(text string) []core.Package {
	pkgs := []core.Package{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 4 {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimSpace(cells[0]), " ")
		if name == "" || name == "Package" {
			continue
		}
		pkgs = append(pkgs, core.Package{Name: name, Version: strings.TrimSpace(cells[1]), Latest: strings.TrimSpace(cells[3])})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs
}
