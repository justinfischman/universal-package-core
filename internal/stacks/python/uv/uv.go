// Package uv is the uv package manager for Python. It routes every capability
// to the uv binary; uv owns pyproject.toml, uv.lock and script metadata, and
// upc never writes them (ADR-0002, ADR-0003).
package uv

import (
	"io/fs"
	"runtime"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// ID is the package manager id; Binary is the executable it wraps.
const (
	ID     = "uv"
	Binary = "uv"
)

// UV implements core.PackageManager and core.Diagnoser.
type UV struct {
	core.BasePackageManager
}

var (
	_ core.PackageManager = (*UV)(nil)
	_ core.Diagnoser      = (*UV)(nil)
	_ core.DetectRanker   = (*UV)(nil)
)

// Detection ranks: a lockfile outranks tool configuration, which outranks a
// bare script; pip's requirements.txt ranks below all of them.
const (
	rankLockfile = 300
	rankConfig   = 200
	rankScript   = 150
)

// New builds uv with the install command for the running platform.
func New() (*UV, error) { return newFor(runtime.GOOS) }

func newFor(goos string) (*UV, error) {
	u := &UV{BasePackageManager: core.NewBasePackageManager(ID, python.ID, tool(goos), "uv.lock")}
	return u, u.declare()
}

// tool describes the binary and how to install it on goos.
func tool(goos string) core.Tool { return spec.Tool.Tool(goos) }

// declare records every capability (ADR-0003 parity).
func (u *UV) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     u.initProject,
		core.CapInstall:  u.install,
		core.CapAdd:      u.add,
		core.CapRemove:   u.remove,
		core.CapList:     u.list,
		core.CapUpdate:   u.update,
		core.CapOutdated: u.outdated,
		core.CapRun:      u.runScript,
		core.CapAudit:    u.audit,
		core.CapBuild:    u.build,
		core.CapPublish:  u.publish,
	}
	filled := map[core.Capability]core.Handler{
		core.CapTest: u.test,
		core.CapLint: u.lint,
		core.CapUp:   u.up,
		core.CapDown: u.down,
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = u.Declare(c, core.Native(), h)
		} else if h, ok := filled[c]; ok {
			err = u.Declare(c, core.Filled(), h)
		} else {
			err = u.Declare(c, core.Unsupported("not supported by uv"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Detect implements core.PackageManager: a uv.lock, a [tool.uv] table in
// pyproject.toml, or (without a pyproject.toml) a standalone script with inline
// metadata, which uv manages natively. Marker files only; user config is
// applied by the caller.
func (u *UV) Detect(fsys fs.FS) bool {
	if u.BasePackageManager.Detect(fsys) {
		return true
	}
	p, err := python.ReadProject(fsys)
	if err == nil {
		return p.HasToolUv
	}
	if _, statErr := fs.Stat(fsys, python.PyprojectFilename); statErr == nil {
		return false // unreadable pyproject.toml: not enough to claim the directory
	}
	scripts, err := python.ScriptFiles(fsys)
	return err == nil && len(scripts) > 0
}

// DetectRank implements core.DetectRanker.
func (u *UV) DetectRank(fsys fs.FS) int {
	switch {
	case !u.Detect(fsys):
		return 0
	case u.BasePackageManager.Detect(fsys):
		return rankLockfile
	}
	if p, err := python.ReadProject(fsys); err == nil && p.HasToolUv {
		return rankConfig
	}
	return rankScript
}
