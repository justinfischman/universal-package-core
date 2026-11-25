package pnpm

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
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

// filterFlags renders `--filter <member name>` for each group member; the
// subset is not remembered, so it is passed on every command (ADR-0015).
func filterFlags(groups []string) []string {
	var out []string
	for _, name := range shared.MemberNames(groups) {
		out = append(out, "--filter", name)
	}
	return out
}

// install routes to `pnpm install`, or for custom groups to
// `pnpm install --filter <member>...` (ADR-0015 option B).
func (pn *Pnpm) install(ctx context.Context, req core.Request) (core.Result, error) {
	custom, err := shared.CustomGroups(req.AllGroups())
	if err != nil {
		return core.Result{}, err
	}
	if len(custom) == 0 {
		return pn.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), shared.FullInstallSummary(req.AllGroups()), "install dependencies")
	}
	if err := requireManifest(req); err != nil {
		return core.Result{}, err
	}
	if err := shared.RequireMembers(invoke.DirFS(req.Dir), custom); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"install"}, filterFlags(custom), req.Args)
	list := strings.Join(custom, ", ")
	return pn.stream(ctx, req, args, "dependency groups installed: "+list, "install dependency groups "+list)
}

// groupFlags returns the flags that place a package in req.Group: a manager
// flag for dev, optional and peer, otherwise `--filter` for the group's
// member. add creates the member (and registers it as a workspace) when
// missing; remove needs it.
func (pn *Pnpm) groupFlags(req core.Request, mustExist bool) ([]string, error) {
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
		if err := shared.EnsureMember(pn.fsys, req.Dir, req.Group); err != nil {
			return nil, err
		}
		if err := pn.ensureWorkspaceFile(req.Dir); err != nil {
			return nil, err
		}
	}
	return filterFlags([]string{req.Group}), nil
}

// add routes to `pnpm add [-D|-O|--save-peer|--filter m] pkgs... [args...]`.
func (pn *Pnpm) add(ctx context.Context, req core.Request) (core.Result, error) {
	return pn.change(ctx, req, "add", "added", "add")
}

// remove routes to `pnpm remove`, with the same shape as add. For a built-in
// group pnpm removes the package from whichever group holds it, so the group
// is validated but its flag is not passed.
func (pn *Pnpm) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return pn.change(ctx, req, "remove", "removed", "remove")
}

func (pn *Pnpm) change(ctx context.Context, req core.Request, verb, past, plan string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := pn.groupFlags(req, verb == "remove")
	if err != nil {
		return core.Result{}, err
	}
	if verb == "remove" && shared.IsBuiltinGroup(req.Group) {
		group = nil
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{verb}, group, req.Packages, req.Args)
	return pn.stream(ctx, req, args, past+" "+list, plan+" "+list)
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

// test routes to `pnpm test [args...]`: pnpm runs the project's `test` script and
// hands the arguments to it.
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
