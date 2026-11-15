// Package poetry is the Poetry package manager for Python. It routes the
// dependency, build and publish capabilities to the poetry binary; poetry owns
// pyproject.toml and poetry.lock, and upc never writes them (ADR-0002,
// ADR-0003). test, lint and audit are filled by running the standard tools in
// the project environment through `poetry run`.
package poetry

import (
	"io/fs"
	"runtime"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// ID is the package manager id; Binary is the executable it wraps.
const (
	ID     = "poetry"
	Binary = "poetry"

	lockFile = "poetry.lock"
	toolName = "poetry" // the [tool.poetry] table
)

// Detection ranks: a lockfile outranks tool configuration; both outrank pip's
// requirements.txt.
const (
	rankLockfile = 300
	rankConfig   = 200
)

// Poetry implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type Poetry struct {
	core.BasePackageManager
	fsys fsx.FS
}

var (
	_ core.PackageManager = (*Poetry)(nil)
	_ core.Diagnoser      = (*Poetry)(nil)
	_ core.DetectRanker   = (*Poetry)(nil)
)

// New builds poetry with the install command for the running platform.
func New(fsys fsx.FS) (*Poetry, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*Poetry, error) {
	p := &Poetry{BasePackageManager: core.NewBasePackageManager(ID, python.ID, tool(goos), lockFile), fsys: fsys}
	return p, p.declare()
}

// tool describes the binary and how to install it on goos.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: Binary, VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "poetry"}
		t.InstallHint = "brew install poetry"
	case "windows":
		t.InstallHint = `install pipx, then run: pipx install poetry (see https://python-poetry.org/docs/#installation)`
	default:
		t.InstallHint = "pipx install poetry (see https://python-poetry.org/docs/#installation)"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (p *Poetry) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     p.initProject,
		core.CapInstall:  p.install,
		core.CapAdd:      p.add,
		core.CapRemove:   p.remove,
		core.CapList:     p.list,
		core.CapUpdate:   p.update,
		core.CapOutdated: p.outdated,
		core.CapRun:      p.runScript,
		core.CapBuild:    p.build,
		core.CapPublish:  p.publish,
	}
	filled := map[core.Capability]core.Handler{
		core.CapTest:  p.test,
		core.CapLint:  p.lint,
		core.CapAudit: p.audit,
		core.CapUp:    p.up,
		core.CapDown:  p.down,
	}
	for _, c := range core.AllCapabilities {
		var err error
		switch {
		case native[c] != nil:
			err = p.Declare(c, core.Native(), native[c])
		case filled[c] != nil:
			err = p.Declare(c, core.Filled(), filled[c])
		default:
			err = p.Declare(c, core.Unsupported("not supported by poetry"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Detect implements core.PackageManager: a poetry.lock or a [tool.poetry]
// table in pyproject.toml. Marker files only; user config is applied by the
// caller.
func (p *Poetry) Detect(fsys fs.FS) bool {
	if p.BasePackageManager.Detect(fsys) {
		return true
	}
	proj, err := python.ReadProject(fsys)
	return err == nil && proj.ToolTables[toolName]
}

// DetectRank implements core.DetectRanker.
func (p *Poetry) DetectRank(fsys fs.FS) int {
	switch {
	case p.BasePackageManager.Detect(fsys):
		return rankLockfile
	case p.Detect(fsys):
		return rankConfig
	}
	return 0
}

// Diagnose implements core.Diagnoser: problems poetry itself would trip over.
func (p *Poetry) Diagnose(fsys fs.FS) []core.Finding {
	if !p.Detect(fsys) || p.BasePackageManager.Detect(fsys) {
		return nil
	}
	return []core.Finding{{
		Severity: core.SeverityWarn,
		Message:  "poetry.lock is missing, so dependencies are not locked",
		Fix:      core.CapInstall,
	}}
}
