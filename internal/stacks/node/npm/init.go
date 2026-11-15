package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// initProject creates a project with `npm init`. The script type is a
// directory with a package.json and one runnable file; the package type is a
// TypeScript library and the app type a runnable TypeScript application, both
// with their development dependencies installed. With no
// project type it is plain `npm init`.
func (n *NPM) initProject(ctx context.Context, req core.Request) (core.Result, error) {
	switch req.ProjectType {
	case "script":
		return n.initScript(ctx, req)
	case "package":
		return n.initPackage(ctx, req)
	case "app":
		return n.initApp(ctx, req)
	case "":
		return n.initPlain(ctx, req)
	}
	return core.Result{}, fmt.Errorf("%w: npm cannot scaffold a %q project yet", core.ErrInvalidRequest, req.ProjectType)
}

func (n *NPM) initPlain(ctx context.Context, req core.Request) (core.Result, error) {
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, invoke.Concat([]string{"init", "--yes"}, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("initialized project", "initialize project", nil), nil
}

// initScript runs `npm init`, names the package, adds a start script and writes
// the source file.
func (n *NPM) initScript(ctx context.Context, req core.Request) (result core.Result, err error) {
	file, err := shared.ScriptFileName(req.Script)
	if err != nil {
		return core.Result{}, err
	}
	name, err := shared.PackageNameFor(file)
	if err != nil {
		return core.Result{}, err
	}
	dest := filepath.Join(req.Dir, file)
	if _, err := n.fsys.Stat(dest); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, dest)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	manifest := filepath.Join(req.Dir, node.PackageFilename)
	if !req.DryRun {
		if err := n.seedManifest(manifest, name); err != nil {
			return core.Result{}, err
		}
		defer func() {
			if err != nil {
				_ = n.fsys.Remove(manifest)
			}
		}()
	}
	steps := [][]string{
		{"init", "--yes"},
		{"pkg", "set", "version=0.1.0", "type=module", "engines.node=" + shared.ScriptEngine,
			"scripts.start=" + shared.StartCommand(file)},
		{"pkg", "set", "private=true", "--json"},
		{"pkg", "delete", "main", "scripts.test"},
	}
	for _, args := range steps {
		if _, err := inv.Run(true, args...); err != nil {
			return core.Result{}, err
		}
	}
	if !req.DryRun {
		if err := n.fsys.WriteFileAtomic(dest, []byte(shared.ScriptSource), 0o644); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", file, err)
		}
	}
	return inv.Result("initialized "+file, "initialize "+file, nil), nil
}

// initPackage creates Dir/Name: `npm init`, library fields and scripts, then
// the TypeScript toolchain as development dependencies. A failure removes the
// directory again.
func (n *NPM) initPackage(ctx context.Context, req core.Request) (result core.Result, err error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := n.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := n.fsys.MkdirAll(root, 0o755); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", root, err)
		}
		defer func() {
			if err != nil {
				_ = n.fsys.RemoveAll(root)
			}
		}()
	}
	steps := [][]string{
		{"init", "--yes"},
		{"pkg", "set", "version=0.1.0", "type=module", "main=dist/index.js", "types=dist/index.d.ts",
			"files[]=dist", "scripts.build=tsc", "scripts.test=node --test", "engines.node=" + shared.PackageEngine},
		{"install", "--save-dev", "typescript", "@types/node"},
	}
	for _, args := range steps {
		if _, err := inv.RunAt(root, true, args...); err != nil {
			return core.Result{}, err
		}
	}
	return inv.Result("created package "+req.Name, "create package "+req.Name, nil), nil
}

// seedManifest writes a package.json that holds only the package name, so
// `npm init` keeps it: npm would otherwise derive the name from the directory,
// which is a staging directory while a project is being created.
func (n *NPM) seedManifest(path, name string) error {
	data, err := json.MarshalIndent(map[string]string{"name": name}, "", "  ")
	if err != nil {
		return err
	}
	if _, err := n.fsys.Stat(path); err == nil {
		return fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, path)
	}
	if err := n.fsys.WriteFileAtomic(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return nil
}
