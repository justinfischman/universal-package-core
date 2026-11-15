package uv

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// mode is what kind of Python project a directory holds.
type mode int

const (
	modeProject      mode = iota // pyproject.toml
	modeScript                   // a standalone PEP 723 script
	modeRequirements             // requirements.txt only
)

// target is the resolved subject of a dependency command.
type target struct {
	mode   mode
	script string // script file name, modeScript only
}

// resolve decides whether a command acts on a script, a pyproject project or a
// requirements.txt project. An explicit req.Script always selects script mode.
func resolve(req core.Request) (target, error) {
	fsys := invoke.DirFS(req.Dir)
	if req.Script != "" {
		if err := shared.ValidateName(req.Script); err != nil {
			return target{}, err
		}
		if _, err := fs.Stat(fsys, filepath.ToSlash(req.Script)); err != nil {
			return target{}, fmt.Errorf("%w: script %q not found in %s", core.ErrInvalidRequest, req.Script, orDot(req.Dir))
		}
		return target{mode: modeScript, script: req.Script}, nil
	}
	if _, err := fs.Stat(fsys, python.PyprojectFilename); err == nil {
		return target{mode: modeProject}, nil
	}
	scripts, err := python.ScriptFiles(fsys)
	if err != nil {
		return target{}, fmt.Errorf("inspect %s: %w", orDot(req.Dir), err)
	}
	switch len(scripts) {
	case 1:
		return target{mode: modeScript, script: scripts[0]}, nil
	case 0:
		if _, err := fs.Stat(fsys, "requirements.txt"); err == nil {
			return target{mode: modeRequirements}, nil
		}
		return target{}, fmt.Errorf("%w: no pyproject.toml, requirements.txt or script with inline metadata in %s; run `upc init` or `upc new`", core.ErrInvalidRequest, orDot(req.Dir))
	default:
		return target{}, fmt.Errorf("%w: several scripts found (%s); choose one with --script", core.ErrInvalidRequest, strings.Join(scripts, ", "))
	}
}

func orDot(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

// scriptArgs returns the flags that point uv at a script, or nil.
func (t target) scriptArgs() []string {
	if t.mode == modeScript {
		return []string{"--script", t.script}
	}
	return nil
}
