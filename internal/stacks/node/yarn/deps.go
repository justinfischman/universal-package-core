package yarn

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
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
	custom, err := shared.CustomGroups(req.AllGroups())
	if err != nil {
		return core.Result{}, err
	}
	if len(custom) == 0 {
		return y.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), shared.FullInstallSummary(req.AllGroups()), "install dependencies")
	}
	if err := requireManifest(req); err != nil {
		return core.Result{}, err
	}
	if err := shared.RequireMembers(invoke.DirFS(req.Dir), custom); err != nil {
		return core.Result{}, err
	}
	major, err := yarnMajor(ctx, req.Dir)
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(custom, ", ")
	if major < 4 {
		// ADR-0015: classic (1.x) has no subset install, and `workspaces focus`
		// is built in only from yarn 4 (2.x/3.x need the workspace-tools
		// plugin), so the install is full.
		return y.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args),
			fmt.Sprintf("dependencies installed (yarn %d installs every group, %s included; subset install needs yarn 4)", max(major, 1), list),
			"install dependencies, groups "+list)
	}
	args := invoke.Concat([]string{"workspaces", "focus"}, shared.MemberNames(custom), req.Args)
	return y.stream(ctx, req, args, "dependency groups installed: "+list, "install dependency groups "+list)
}

// groupWorkspace is the `workspace <member>` prefix for a custom group, or nil
// for dev, optional, peer and no group. add creates the member (and registers it
// as a workspace) when missing; remove needs it to exist.
func (y *Yarn) groupWorkspace(req core.Request, mustExist bool) ([]string, error) {
	if req.Group == "" || shared.IsBuiltinGroup(req.Group) {
		return nil, nil
	}
	if err := shared.ValidateGroupName(req.Group); err != nil {
		return nil, err
	}
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	switch {
	case mustExist:
		if err := shared.RequireMembers(invoke.DirFS(req.Dir), []string{req.Group}); err != nil {
			return nil, err
		}
	case !req.DryRun:
		if err := shared.EnsureMember(y.fsys, req.Dir, req.Group); err != nil {
			return nil, err
		}
	}
	return []string{"workspace", shared.MemberName(req.Group)}, nil
}

// add routes to `yarn add [-D|-O|-P] pkgs... [args...]`, or for a custom group
// to `yarn workspace <member> add pkgs...`.
func (y *Yarn) add(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	member, err := y.groupWorkspace(req, false)
	if err != nil {
		return core.Result{}, err
	}
	var group []string
	if member == nil {
		if group, err = groupArgs(req.Group); err != nil {
			return core.Result{}, err
		}
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat(member, []string{"add"}, group, req.Packages, req.Args)
	return y.stream(ctx, req, args, "added "+list, "add "+list)
}

// remove routes to `yarn remove pkgs...`. yarn removes a package from whichever
// built-in group holds it, so that group is validated but not passed; a custom
// group is `yarn workspace <member> remove pkgs...`.
func (y *Yarn) remove(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	member, err := y.groupWorkspace(req, true)
	if err != nil {
		return core.Result{}, err
	}
	if member == nil {
		if _, err := groupArgs(req.Group); err != nil {
			return core.Result{}, err
		}
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat(member, []string{"remove"}, req.Packages, req.Args)
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
	if err := rejectClassicDryRun(verb, req.Args); err != nil {
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
