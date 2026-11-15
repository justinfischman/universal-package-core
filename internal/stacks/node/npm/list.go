package npm

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

// list shows direct dependencies: `npm ls --depth=0 --json`.
func (n *NPM) list(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := n.query(ctx, req, []string{"ls", "--depth=0", "--json"})
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
// `npm outdated --json`.
func (n *NPM) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := n.query(ctx, req, []string{"outdated", "--json"})
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

// query runs a read-only npm command whose JSON upc parses. npm exits 1 for
// findings it still reports on stdout (outdated packages, dependency
// problems), so a failed command with output is returned for the parser to
// judge; one without output is a real failure.
func (n *NPM) query(ctx context.Context, req core.Request, args []string) (string, *invoke.Invocation, error) {
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

// npmError is the error object npm prints in its JSON output.
type npmError struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
}

func (e *npmError) failure(what string) error {
	return fmt.Errorf("npm %s failed: %s: %s", what, e.Code, e.Summary)
}

// ParseList reads `npm ls --json`: the direct dependencies, sorted by name. A
// declared dependency that is not installed has no version.
func ParseList(text string) ([]core.Package, error) {
	if strings.TrimSpace(text) == "" {
		return []core.Package{}, nil
	}
	var doc struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
		Error *npmError `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("parse npm ls output: %w", err)
	}
	if doc.Error != nil {
		return nil, doc.Error.failure("ls")
	}
	pkgs := make([]core.Package, 0, len(doc.Dependencies))
	for name, d := range doc.Dependencies {
		pkgs = append(pkgs, core.Package{Name: name, Version: d.Version})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// outdatedEntry is one package in `npm outdated --json`.
type outdatedEntry struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// ParseOutdated reads `npm outdated --json`: an object keyed by package name.
// A package required from several places is an array of entries; the first is
// used. An empty object or empty output means nothing is outdated.
func ParseOutdated(text string) ([]core.Package, error) {
	if strings.TrimSpace(text) == "" {
		return []core.Package{}, nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("parse npm outdated output: %w", err)
	}
	if raw, ok := doc["error"]; ok {
		var e npmError
		if err := json.Unmarshal(raw, &e); err == nil && e.Code != "" {
			return nil, e.failure("outdated")
		}
	}
	pkgs := make([]core.Package, 0, len(doc))
	for name, raw := range doc {
		entry, err := firstEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("parse npm outdated entry %q: %w", name, err)
		}
		pkgs = append(pkgs, core.Package{Name: name, Version: entry.Current, Latest: entry.Latest})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

func firstEntry(raw json.RawMessage) (outdatedEntry, error) {
	var one outdatedEntry
	if err := json.Unmarshal(raw, &one); err == nil {
		return one, nil
	}
	var many []outdatedEntry
	if err := json.Unmarshal(raw, &many); err != nil {
		return outdatedEntry{}, err
	}
	if len(many) == 0 {
		return outdatedEntry{}, errors.New("empty entry")
	}
	return many[0], nil
}
