package npm

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// groupArgs maps upc's group to npm flags: "dev" is -D, "optional" is -O and
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
	return nil, fmt.Errorf("%w: %q is not a dependency group npm knows (dev, optional, peer)", core.ErrInvalidRequest, group)
}

// install routes to `npm install`.
func (n *NPM) install(ctx context.Context, req core.Request) (core.Result, error) {
	custom, err := shared.CustomGroups(req.AllGroups())
	if err != nil {
		return core.Result{}, err
	}
	if len(custom) == 0 {
		return n.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), "dependencies installed", "install dependencies")
	}
	if err := requireManifest(req); err != nil {
		return core.Result{}, err
	}
	if err := shared.RequireMembers(invoke.DirFS(req.Dir), custom); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"install"}, workspaceFlags(custom), req.Args)
	list := strings.Join(custom, ", ")
	return n.stream(ctx, req, args, "dependency groups installed: "+list, "install dependency groups "+list)
}

// workspaceFlags renders `-w <member dir>` for each group member; the subset is
// not remembered, so it is passed on every command (ADR-0015).
func workspaceFlags(groups []string) []string {
	var out []string
	for _, dir := range shared.GroupDirs(groups) {
		out = append(out, "-w", dir)
	}
	return out
}

// groupFlags returns the flags that place a package in req.Group: a manager flag
// for dev, optional and peer, otherwise `-w` for the group's member. add creates
// the member (and registers it as a workspace) when missing; remove needs it.
func (n *NPM) groupFlags(req core.Request, mustExist bool) ([]string, error) {
	if req.Group == "" || shared.IsBuiltinGroup(req.Group) {
		return groupArgs(req.Group)
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
		if err := shared.EnsureMember(n.fsys, req.Dir, req.Group); err != nil {
			return nil, err
		}
	}
	return workspaceFlags([]string{req.Group}), nil
}

// add routes to `npm install [-D|-O|--save-peer] pkgs... [args...]`.
func (n *NPM) add(ctx context.Context, req core.Request) (core.Result, error) {
	return n.change(ctx, req, "install", "added", "add")
}

// remove routes to `npm uninstall`, with the same shape as add.
func (n *NPM) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return n.change(ctx, req, "uninstall", "removed", "remove")
}

func (n *NPM) change(ctx context.Context, req core.Request, verb, past, plan string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := n.groupFlags(req, verb == "uninstall")
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{verb}, group, req.Packages, req.Args)
	return n.stream(ctx, req, args, past+" "+list, plan+" "+list)
}

// update routes to `npm update [pkgs...]`.
func (n *NPM) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"update"}, req.Packages, req.Args)
	return n.stream(ctx, req, args, "dependencies updated", "update dependencies")
}

// audit routes to `npm audit`.
func (n *NPM) audit(ctx context.Context, req core.Request) (core.Result, error) {
	return n.stream(ctx, req, invoke.Concat([]string{"audit"}, req.Args), "done: audit dependencies", "audit dependencies")
}

// publish routes to `npm publish`.
func (n *NPM) publish(ctx context.Context, req core.Request) (core.Result, error) {
	return n.stream(ctx, req, invoke.Concat([]string{"publish"}, req.Args), "done: publish the project", "publish the project")
}

// test routes to `npm test [-- args...]`.
func (n *NPM) test(ctx context.Context, req core.Request) (core.Result, error) {
	return n.stream(ctx, req, withPassthrough([]string{"test"}, req.Args), "done: run the tests", "run the tests")
}

// withPassthrough appends args after `--` so npm hands them to the script.
func withPassthrough(base, args []string) []string {
	if len(args) == 0 {
		return base
	}
	return invoke.Concat(base, []string{"--"}, args)
}

// stream runs one npm command whose output is for the user, not for upc.
func (n *NPM) stream(ctx context.Context, req core.Request, args []string, done, planned string) (core.Result, error) {
	inv, err := begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, args...); err != nil {
		return core.Result{}, err
	}
	return inv.Result(done, planned, nil), nil
}
