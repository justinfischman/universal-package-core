package pnpm

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// groupArgs maps upc's group to pnpm flags: "dev" is -D, "optional" is -O and
// "peer" is --save-peer.
func groupArgs(group string) ([]string, error) {
	switch group {
	case "":
		return nil, nil
	case "dev":
		return []string{"-D"}, nil
	case "optional":
		return []string{"-O"}, nil
	case "peer":
		return []string{"--save-peer"}, nil
	}
	return nil, fmt.Errorf("%w: %q is not a dependency group pnpm knows (dev, optional, peer)", core.ErrInvalidRequest, group)
}

// install routes to `pnpm install`.
func (pn *Pnpm) install(ctx context.Context, req core.Request) (core.Result, error) {
	if len(req.AllGroups()) > 0 {
		return core.Result{}, fmt.Errorf("%w: pnpm installs every group; groups apply to add and remove (subset install by group is not implemented)", core.ErrInvalidRequest)
	}
	return pn.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), "dependencies installed", "install dependencies")
}

// add routes to `pnpm add [-D|-O|--save-peer] pkgs... [args...]`.
func (pn *Pnpm) add(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := groupArgs(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"add"}, group, req.Packages, req.Args)
	return pn.stream(ctx, req, args, "added "+list, "add "+list)
}

// remove routes to `pnpm remove pkgs...`. pnpm removes a package from
// whichever group holds it, so the group is validated but not passed.
func (pn *Pnpm) remove(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	if _, err := groupArgs(req.Group); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"remove"}, req.Packages, req.Args)
	return pn.stream(ctx, req, args, "removed "+list, "remove "+list)
}

// update routes to `pnpm update [pkgs...]`.
func (pn *Pnpm) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"update"}, req.Packages, req.Args)
	return pn.stream(ctx, req, args, "dependencies updated", "update dependencies")
}

// audit routes to `pnpm audit`.
func (pn *Pnpm) audit(ctx context.Context, req core.Request) (core.Result, error) {
	return pn.stream(ctx, req, invoke.Concat([]string{"audit"}, req.Args), "done: audit dependencies", "audit dependencies")
}

// publish routes to `pnpm publish`.
func (pn *Pnpm) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return pn.stream(ctx, req, invoke.Concat([]string{"publish"}, req.Args), "done: publish the project", "publish the project")
}

// test routes to `pnpm test [args...]`: pnpm hands the arguments to the script.
func (pn *Pnpm) test(ctx context.Context, req core.Request) (core.Result, error) {
	return pn.stream(ctx, req, invoke.Concat([]string{"test"}, req.Args), "done: run the tests", "run the tests")
}

// stream runs one pnpm command whose output is for the user, not for upc.
func (pn *Pnpm) stream(ctx context.Context, req core.Request, args []string, done, planned string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result(done, planned, nil), nil
}
