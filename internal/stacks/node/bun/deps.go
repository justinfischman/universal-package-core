package bun

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// groupArgs maps upc's group to bun flags: "dev" is --dev, "optional" is
// --optional and "peer" is --peer.
func groupArgs(group string) ([]string, error) {
	switch group {
	case "":
		return nil, nil
	case "dev":
		return []string{"--dev"}, nil
	case "optional":
		return []string{"--optional"}, nil
	case "peer":
		return []string{"--peer"}, nil
	}
	return nil, fmt.Errorf("%w: %q is not a dependency group bun knows (dev, optional, peer)", core.ErrInvalidRequest, group)
}

// install routes to `bun install`.
func (bn *Bun) install(ctx context.Context, req core.Request) (core.Result, error) {
	if len(req.AllGroups()) > 0 {
		return core.Result{}, fmt.Errorf("%w: bun installs every group; groups apply to add and remove (subset install by group is not implemented)", core.ErrInvalidRequest)
	}
	return bn.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), "dependencies installed", "install dependencies")
}

// add routes to `bun add [--dev|--optional|--peer] pkgs... [args...]`.
func (bn *Bun) add(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := groupArgs(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"add"}, group, req.Packages, req.Args)
	return bn.stream(ctx, req, args, "added "+list, "add "+list)
}

// remove routes to `bun remove pkgs...`. bun removes a package from whichever
// group holds it, so the group is validated but not passed.
func (bn *Bun) remove(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	if _, err := groupArgs(req.Group); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"remove"}, req.Packages, req.Args)
	return bn.stream(ctx, req, args, "removed "+list, "remove "+list)
}

// update routes to `bun update [pkgs...]`.
func (bn *Bun) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"update"}, req.Packages, req.Args)
	return bn.stream(ctx, req, args, "dependencies updated", "update dependencies")
}

// audit routes to `bun audit`.
func (bn *Bun) audit(ctx context.Context, req core.Request) (core.Result, error) {
	return bn.stream(ctx, req, invoke.Concat([]string{"audit"}, req.Args), "done: audit dependencies", "audit dependencies")
}

// publish routes to `bun publish`.
func (bn *Bun) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return bn.stream(ctx, req, invoke.Concat([]string{"publish"}, req.Args), "done: publish the project", "publish the project")
}

// test routes to `bun run test [args...]`: the project's `test` script. Plain
// `bun test` would start bun's own test runner and ignore that script.
func (bn *Bun) test(ctx context.Context, req core.Request) (core.Result, error) {
	return bn.stream(ctx, req, invoke.Concat([]string{"run", "test"}, req.Args), "done: run the tests", "run the tests")
}

// stream runs one bun command whose output is for the user, not for upc.
func (bn *Bun) stream(ctx context.Context, req core.Request, args []string, done, planned string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result(done, planned, nil), nil
}
