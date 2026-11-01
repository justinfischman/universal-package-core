package yarn

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// list shows direct dependencies: `yarn list --depth=0 --json`, narrowed to
// the names package.json declares (yarn flattens transitive packages into the
// same list).
func (y *Yarn) list(ctx context.Context, req core.Request) (core.Result, error) {
	direct, err := directDependencies(req)
	if err != nil {
		return core.Result{}, err
	}
	out, inv, err := y.query(ctx, req, core.CapList, []string{"list", "--depth=0", "--json"})
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "list dependencies", nil), nil
	}
	pkgs, err := ParseList(out, direct)
	if err != nil {
		return core.Result{}, err
	}
	summary := shared.PackageSummary(pkgs)
	if summary == "" {
		summary = "no dependencies"
	}
	return inv.Result(summary, "", pkgs), nil
}

// outdated shows direct dependencies that have a newer version available:
// `yarn outdated --json`.
func (y *Yarn) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	out, inv, err := y.query(ctx, req, core.CapOutdated, []string{"outdated", "--json"})
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
	stale, summary := shared.OutdatedPackages(pkgs)
	if summary == "" {
		summary = "all dependencies are up to date"
	}
	return inv.Result(summary, "", stale), nil
}

// query runs a read-only yarn command whose JSON upc parses. yarn outdated
// exits 1 when it has findings, so a failed command with output is returned
// for the parser to judge; one without output is a real failure. Berry has
// neither report.
func (y *Yarn) query(ctx context.Context, req core.Request, cap core.Capability, args []string) (string, *shared.Invocation, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return "", nil, err
	}
	berry, err := isBerry(ctx, req.Dir)
	if err != nil {
		return "", inv, err
	}
	if berry {
		return "", inv, berryUnsupported(cap)
	}
	out, err := inv.Run(false, shared.Concat(args, req.Args)...)
	var exit *execx.ExitError
	if err != nil && (!errors.As(err, &exit) || strings.TrimSpace(out.Stdout) == "") {
		return "", inv, err
	}
	return out.Stdout, inv, nil
}

// directDependencies is the names package.json declares as dependencies,
// devDependencies or optionalDependencies.
func directDependencies(req core.Request) ([]string, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	data, err := fs.ReadFile(shared.DirFS(req.Dir), node.PackageFilename)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Dependencies         map[string]json.RawMessage `json:"dependencies"`
		DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
		OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%w: package.json: %v", core.ErrInvalidRequest, err)
	}
	seen := map[string]bool{}
	for _, group := range []map[string]json.RawMessage{doc.Dependencies, doc.DevDependencies, doc.OptionalDependencies} {
		for name := range group {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// event is one line of yarn's --json output.
type event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// events reads yarn's newline-delimited JSON. A line of type "error" is the
// failure yarn reports (it prints it on stdout in --json mode).
func events(text, what string) ([]event, error) {
	var out []event
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("parse yarn %s output: %w", what, err)
		}
		if e.Type == "error" {
			var msg string
			_ = json.Unmarshal(e.Data, &msg)
			first, _, _ := strings.Cut(msg, "\n")
			return nil, fmt.Errorf("yarn %s failed: %s", what, first)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse yarn %s output: %w", what, err)
	}
	return out, nil
}

// ParseList reads `yarn list --depth=0 --json` and returns the packages named
// in direct, sorted by name. A declared dependency that is not installed has
// no version.
func ParseList(text string, direct []string) ([]core.Package, error) {
	evs, err := events(text, "list")
	if err != nil {
		return nil, err
	}
	versions := map[string]string{}
	for _, e := range evs {
		if e.Type != "tree" {
			continue
		}
		var tree struct {
			Trees []struct {
				Name string `json:"name"`
			} `json:"trees"`
		}
		if err := json.Unmarshal(e.Data, &tree); err != nil {
			return nil, fmt.Errorf("parse yarn list output: %w", err)
		}
		for _, t := range tree.Trees {
			name, version := splitSpec(t.Name)
			versions[name] = version
		}
	}
	pkgs := make([]core.Package, 0, len(direct))
	for _, name := range direct {
		pkgs = append(pkgs, core.Package{Name: name, Version: versions[name]})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// splitSpec splits "name@version", keeping the scope of "@scope/name@version".
func splitSpec(spec string) (name, version string) {
	i := strings.LastIndex(spec, "@")
	if i <= 0 {
		return spec, ""
	}
	return spec[:i], spec[i+1:]
}

// ParseOutdated reads `yarn outdated --json`: a table event whose head names
// the columns. Empty output means nothing is outdated.
func ParseOutdated(text string) ([]core.Package, error) {
	evs, err := events(text, "outdated")
	if err != nil {
		return nil, err
	}
	pkgs := []core.Package{}
	for _, e := range evs {
		if e.Type != "table" {
			continue
		}
		var table struct {
			Head []string   `json:"head"`
			Body [][]string `json:"body"`
		}
		if err := json.Unmarshal(e.Data, &table); err != nil {
			return nil, fmt.Errorf("parse yarn outdated output: %w", err)
		}
		col := map[string]int{}
		for i, h := range table.Head {
			col[h] = i
		}
		if _, ok := col["Package"]; !ok {
			return nil, errors.New("parse yarn outdated output: table has no Package column")
		}
		name, cur, latest := col["Package"], col["Current"], col["Latest"]
		for _, row := range table.Body {
			if len(row) <= name || len(row) <= cur || len(row) <= latest {
				return nil, errors.New("parse yarn outdated output: short table row")
			}
			pkgs = append(pkgs, core.Package{Name: row[name], Version: row[cur], Latest: row[latest]})
		}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
