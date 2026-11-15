package yarn

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// groupArgs maps upc's group to yarn flags: "dev" is -D, "optional" is -O and
// "peer" is -P (the same in classic and berry).
func groupArgs(group string) ([]string, error) {
	switch group {
	case "":
		return nil, nil
	case "dev":
		return []string{"-D"}, nil
	case "optional":
		return []string{"-O"}, nil
	case "peer":
		return []string{"-P"}, nil
	}
	return nil, fmt.Errorf("%w: %q is not a dependency group yarn knows (dev, optional, peer)", core.ErrInvalidRequest, group)
}

// install routes to `yarn install`.
func (y *Yarn) install(ctx context.Context, req core.Request) (core.Result, error) {
	if len(req.AllGroups()) > 0 {
		return core.Result{}, fmt.Errorf("%w: yarn installs every group; groups apply to add and remove (subset install by group is not implemented)", core.ErrInvalidRequest)
	}
	return y.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), "dependencies installed", "install dependencies")
}

// add routes to `yarn add [-D|-O|-P] pkgs... [args...]`.
func (y *Yarn) add(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := groupArgs(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"add"}, group, req.Packages, req.Args)
	return y.stream(ctx, req, args, "added "+list, "add "+list)
}

// remove routes to `yarn remove pkgs...`. yarn removes a package from whichever
// group holds it, so the group is validated but not passed.
func (y *Yarn) remove(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	if _, err := groupArgs(req.Group); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{"remove"}, req.Packages, req.Args)
	return y.stream(ctx, req, args, "removed "+list, "remove "+list)
}

// update routes to `yarn upgrade [pkgs...]` (berry: `yarn up`).
func (y *Yarn) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	verb, err := y.verb(ctx, req, "upgrade", "up")
	if err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat(verb, req.Packages, req.Args)
	return y.stream(ctx, req, args, "dependencies updated", "update dependencies")
}

// audit routes to `yarn audit` (berry: `yarn npm audit`).
func (y *Yarn) audit(ctx context.Context, req core.Request) (core.Result, error) {
	verb, err := y.verb(ctx, req, "audit", "npm", "audit")
	if err != nil {
		return core.Result{}, err
	}
	return y.stream(ctx, req, invoke.Concat(verb, req.Args), "done: audit dependencies", "audit dependencies")
}

// publish routes to `yarn publish` (berry: `yarn npm publish`).
func (y *Yarn) publish(ctx context.Context, req core.Request) (core.Result, error) {
	verb, err := y.verb(ctx, req, "publish", "npm", "publish")
	if err != nil {
		return core.Result{}, err
	}
	return y.stream(ctx, req, invoke.Concat(verb, req.Args), "done: publish the project", "publish the project")
}

// test routes to `yarn test [args...]`; yarn hands trailing arguments to the script.
func (y *Yarn) test(ctx context.Context, req core.Request) (core.Result, error) {
	return y.stream(ctx, req, invoke.Concat([]string{"test"}, req.Args), "done: run the tests", "run the tests")
}

// verb returns the command words for the project's yarn flavor: classic is the
// first word, berry the rest.
func (y *Yarn) verb(ctx context.Context, req core.Request, classic string, berry ...string) ([]string, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	isB, err := isBerry(ctx, req.Dir)
	if err != nil {
		return nil, err
	}
	if isB {
		return berry, nil
	}
	return []string{classic}, nil
}

// stream runs one yarn command whose output is for the user, not for upc.
func (y *Yarn) stream(ctx context.Context, req core.Request, args []string, done, planned string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result(done, planned, nil), nil
}
