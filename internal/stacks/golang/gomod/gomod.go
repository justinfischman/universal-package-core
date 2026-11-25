// Package gomod is a SPIKE of the Go modules manager (ADR-0019). It routes
// capabilities to the go binary, which owns go.mod and go.sum. It is not
// registered anywhere.
package gomod

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/golang"
)

// ID is the manager id; Binary is the executable it wraps.
const (
	ID     = "gomod"
	Binary = "go"
)

// listFormat prints "path version" for each direct, non-main module.
const listFormat = `{{if and (not .Main) (not .Indirect)}}{{.Path}} {{.Version}}{{end}}`

// GoMod implements core.PackageManager.
type GoMod struct{ core.BasePackageManager }

var _ core.PackageManager = (*GoMod)(nil)

// New builds the Go modules manager.
func New() (*GoMod, error) {
	tool := core.Tool{
		Binary:      Binary,
		VersionArgs: []string{"version"},
		InstallHint: "install Go from https://go.dev/dl/",
	}
	// go.sum is a checksum database, not a lock file, so only go.mod is a marker.
	g := &GoMod{core.NewBasePackageManager(ID, golang.ID, tool, golang.ManifestFilename)}
	return g, g.declare()
}

func (g *GoMod) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:    g.initModule,
		core.CapInstall: g.fixed("download modules", "mod", "download"),
		core.CapAdd:     g.change(false),
		core.CapRemove:  g.change(true),
		core.CapList:    g.list,
		core.CapUpdate:  g.fixed("update dependencies", "get", "-u", "./..."),
		core.CapRun:     g.run,
		core.CapBuild:   g.fixed("build", "build", "./..."),
		core.CapTest:    g.fixed("run tests", "test", "./..."),
		core.CapLint:    g.fixed("vet", "vet", "./..."),
	}
	reasons := map[core.Capability]string{
		core.CapOutdated: "go has no machine-readable outdated report that upc parses yet",
		core.CapAudit:    "vulnerability scanning is the separate govulncheck tool",
		core.CapUp:       "go has no service runner",
		core.CapDown:     "go has no service runner",
		core.CapPublish:  "Go modules are published by pushing a version tag",
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = g.Declare(c, core.Native(), h)
		} else {
			err = g.Declare(c, core.Unsupported(reasons[c]), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// stream runs `go args...` with output shown to the user.
func (g *GoMod) stream(ctx context.Context, req core.Request, done, planned string, args ...string) (core.Result, error) {
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result(done, planned, nil), nil
}

// fixed builds a handler for a capability that is one fixed go command plus passthrough args.
func (g *GoMod) fixed(what string, args ...string) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Result, error) {
		return g.stream(ctx, req, what+" finished", what, invoke.Concat(args, req.Args)...)
	}
}

// initModule runs `go mod init <Name>`; Go modules are named by import path.
func (g *GoMod) initModule(ctx context.Context, req core.Request) (core.Result, error) {
	if req.Name == "" {
		return core.Result{}, fmt.Errorf("%w: go mod init needs a module path (Name)", core.ErrInvalidRequest)
	}
	return g.stream(ctx, req, "initialized "+req.Name, "initialize "+req.Name, "mod", "init", req.Name)
}

// change adds (`go get path`) or removes (`go get path@none`) modules. Go has
// no dependency groups, so a group is an invalid request, not silently ignored.
func (g *GoMod) change(remove bool) core.Handler {
	return func(ctx context.Context, req core.Request) (core.Result, error) {
		if err := req.RequirePackages(); err != nil {
			return core.Result{}, err
		}
		if req.Group != "" {
			return core.Result{}, fmt.Errorf("%w: go has no dependency groups, got %q", core.ErrInvalidRequest, req.Group)
		}
		specs, verb := req.Packages, "added"
		if remove {
			specs, verb = make([]string, len(req.Packages)), "removed"
			for i, p := range req.Packages {
				specs[i] = p + "@none"
			}
		}
		list := strings.Join(req.Packages, " ")
		return g.stream(ctx, req, verb+" "+list, verb+" "+list, invoke.Concat([]string{"get"}, specs, req.Args)...)
	}
}

// run executes `go run . [args]`; Go has no named scripts, so Script is not accepted.
func (g *GoMod) run(ctx context.Context, req core.Request) (core.Result, error) {
	if req.Script != "" {
		return core.Result{}, fmt.Errorf("%w: go has no named scripts, got %q", core.ErrInvalidRequest, req.Script)
	}
	return g.stream(ctx, req, "run finished", "run the main package", invoke.Concat([]string{"run", "."}, req.Args)...)
}

// list reports the direct dependencies.
func (g *GoMod) list(ctx context.Context, req core.Request) (core.Result, error) {
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	out, err := inv.Run(false, "list", "-m", "-f", listFormat, "all")
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return inv.Result("", "list dependencies", nil), nil
	}
	pkgs := ParseList(out.Stdout)
	return inv.Result(fmt.Sprintf("%d direct dependencies", len(pkgs)), "", pkgs), nil
}

// ParseList reads the lines printed with listFormat; blank lines are skipped.
func ParseList(stdout string) []core.Package {
	var pkgs []core.Package
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		p := core.Package{Name: fields[0]}
		if len(fields) > 1 {
			p.Version = fields[1]
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}
