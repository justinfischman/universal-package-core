package pnpm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// manifest is the package.json a new project starts from. pnpm has no
// `pkg set`, so upc seeds every field it needs and does not run `pnpm init`,
// which refuses an existing package.json (ADR-0010 addendum). Field order is the file's order.
type manifest struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Private bool              `json:"private,omitempty"`
	Type    string            `json:"type"`
	Main    string            `json:"main,omitempty"`
	Types   string            `json:"types,omitempty"`
	Files   []string          `json:"files,omitempty"`
	Scripts map[string]string `json:"scripts,omitempty"`
	Engines map[string]string `json:"engines"`
}

// initProject creates a project with `pnpm init`. The script type is a
// directory with a package.json and one runnable file; the package type is a
// TypeScript library and the app type a runnable TypeScript application, both
// with their development dependencies installed. With no
// project type it is plain `pnpm init --yes`.
func (pn *Pnpm) initProject(ctx context.Context, req core.Request) (core.Result, error) {
	switch req.ProjectType {
	case "script":
		return pn.initScript(ctx, req)
	case "package":
		return pn.initPackage(ctx, req)
	case "app":
		return pn.initApp(ctx, req)
	case "":
		return pn.initPlain(ctx, req)
	}
	return core.Result{}, fmt.Errorf("%w: pnpm cannot scaffold a %q project yet", core.ErrInvalidRequest, req.ProjectType)
}

func (pn *Pnpm) initPlain(ctx context.Context, req core.Request) (core.Result, error) {
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if _, err := inv.Run(true, invoke.Concat([]string{"init"}, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return inv.Result("initialized project", "initialize project", nil), nil
}

// initScript seeds a private manifest with a start script, installs
// (which writes the lock file) and writes the source file.
func (pn *Pnpm) initScript(ctx context.Context, req core.Request) (result core.Result, err error) {
	file, err := shared.ScriptFileName(req.Script)
	if err != nil {
		return core.Result{}, err
	}
	name, err := shared.PackageNameFor(file)
	if err != nil {
		return core.Result{}, err
	}
	dest := filepath.Join(req.Dir, file)
	if _, err := pn.fsys.Stat(dest); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, dest)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	path := filepath.Join(req.Dir, node.PackageFilename)
	if !req.DryRun {
		seedErr := pn.seed(path, manifest{
			Name: name, Version: "0.1.0", Private: true, Type: "module",
			Scripts: map[string]string{"start": shared.StartCommand(file)},
			Engines: map[string]string{"node": shared.ScriptEngine},
		})
		if seedErr != nil {
			return core.Result{}, seedErr
		}
		defer func() {
			if err != nil {
				_ = pn.fsys.Remove(path)
			}
		}()
	}
	if _, err := inv.Run(true, "install"); err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := pn.fsys.WriteFileAtomic(dest, []byte(shared.ScriptSource), 0o644); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", file, err)
		}
	}
	return inv.Result("initialized "+file, "initialize "+file, nil), nil
}

// initPackage creates Dir/Name: a seeded library manifest, then
// the TypeScript toolchain as development dependencies. A failure removes the
// directory again.
func (pn *Pnpm) initPackage(ctx context.Context, req core.Request) (result core.Result, err error) {
	if err := shared.ValidateProjectName(req.Name); err != nil {
		return core.Result{}, err
	}
	root := filepath.Join(req.Dir, req.Name)
	if _, err := pn.fsys.Stat(root); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, root)
	}
	inv, err := invoke.Begin(ctx, req, Binary)
	if err != nil {
		return core.Result{}, err
	}
	if !req.DryRun {
		if err := pn.fsys.MkdirAll(root, 0o755); err != nil {
			return core.Result{}, fmt.Errorf("create %s: %w", root, err)
		}
		defer func() {
			if err != nil {
				_ = pn.fsys.RemoveAll(root)
			}
		}()
		seedErr := pn.seed(filepath.Join(root, node.PackageFilename), manifest{
			Name: req.Name, Version: "0.1.0", Type: "module", Main: "dist/index.js", Types: "dist/index.d.ts",
			Files:   []string{"dist"},
			Scripts: map[string]string{"build": "tsc", "test": "node --test"},
			Engines: map[string]string{"node": shared.PackageEngine},
		})
		if seedErr != nil {
			return core.Result{}, seedErr
		}
	}
	steps := [][]string{
		{"add", "-D", "typescript", "@types/node"},
	}
	for _, args := range steps {
		if _, err := inv.RunAt(root, true, args...); err != nil {
			return core.Result{}, err
		}
	}
	return inv.Result("created package "+req.Name, "create package "+req.Name, nil), nil
}

// seed writes the starting package.json, refusing to overwrite one.
func (pn *Pnpm) seed(path string, m manifest) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep ">=22" readable
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	if _, err := pn.fsys.Stat(path); err == nil {
		return fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, path)
	}
	if err := pn.fsys.WriteFileAtomic(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return nil
}
