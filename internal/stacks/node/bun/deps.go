package bun

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
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

// filterFlags renders `--filter <member name>` for each group member; the
// subset is not remembered, so it is passed on every command (ADR-0015).
func filterFlags(groups []string) []string {
	var out []string
	for _, name := range shared.MemberNames(groups) {
		out = append(out, "--filter", name)
	}
	return out
}

// install routes to `bun install`, or for custom groups to
// `bun install --filter <member>...` (ADR-0015 option B).
func (bn *Bun) install(ctx context.Context, req core.Request) (core.Result, error) {
	custom, err := shared.CustomGroups(req.AllGroups())
	if err != nil {
		return core.Result{}, err
	}
	if len(custom) == 0 {
		return bn.stream(ctx, req, invoke.Concat([]string{"install"}, req.Args), shared.FullInstallSummary(req.AllGroups()), "install dependencies")
	}
	if err := requireManifest(req); err != nil {
		return core.Result{}, err
	}
	if err := shared.RequireMembers(invoke.DirFS(req.Dir), custom); err != nil {
		return core.Result{}, err
	}
	args := invoke.Concat([]string{"install"}, filterFlags(custom), req.Args)
	list := strings.Join(custom, ", ")
	return bn.stream(ctx, req, args, "dependency groups installed: "+list, "install dependency groups "+list)
}

// groupFlags returns the flags that place a package in req.Group: a manager
// flag for dev, optional and peer, otherwise `--filter` for the group's
// member. add creates the member (and registers it as a workspace) when
// missing; remove needs it.
func (bn *Bun) groupFlags(req core.Request, mustExist bool) ([]string, error) {
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
		if err := shared.EnsureMember(bn.fsys, req.Dir, req.Group); err != nil {
			return nil, err
		}
	}
	return filterFlags([]string{req.Group}), nil
}

// add routes to `bun add [--dev|--optional|--peer|--filter m] pkgs... [args...]`.
func (bn *Bun) add(ctx context.Context, req core.Request) (core.Result, error) {
	return bn.change(ctx, req, "add", "added", "add")
}

// remove routes to `bun remove`, with the same shape as add. For a built-in
// group bun removes the package from whichever group holds it, so the group
// is validated but its flag is not passed.
func (bn *Bun) remove(ctx context.Context, req core.Request) (core.Result, error) {
	return bn.change(ctx, req, "remove", "removed", "remove")
}

func (bn *Bun) change(ctx context.Context, req core.Request, verb, past, plan string) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	group, err := bn.groupFlags(req, verb == "remove")
	if err != nil {
		return core.Result{}, err
	}
	if verb == "remove" && shared.IsBuiltinGroup(req.Group) {
		group = nil
	}
	list := strings.Join(req.Packages, " ")
	args := invoke.Concat([]string{verb}, group, req.Packages, req.Args)
	return bn.stream(ctx, req, args, past+" "+list, plan+" "+list)
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
