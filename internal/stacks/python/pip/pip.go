// Package pip is the pip package manager for Python. pip has no project
// manifest, so upc fills the gaps (docs/ARCHITECTURE.md, Phase 3): requirements.txt is the manifest
// and upc edits it exactly, idempotently and atomically; a project-local .venv
// holds the environment; test, lint, build, audit and publish run the standard
// Python tools inside it.
package pip

import (
	"io/fs"
	"runtime"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// ID is the package manager id. Binary is the interpreter upc starts to create
// the environment; every other command runs the environment's own python.
const (
	ID           = "pip"
	Binary       = "python3"
	manifestFile = "requirements.txt"
)

// Detection ranks, higher wins when several managers match a directory.
const rankRequirements = 100

// PIP implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type PIP struct {
	core.BasePackageManager
	fsys fsx.FS
	goos string
}

var (
	_ core.PackageManager = (*PIP)(nil)
	_ core.Diagnoser      = (*PIP)(nil)
	_ core.DetectRanker   = (*PIP)(nil)
)

// New builds pip for the running platform. fsys is where requirements files are
// read and atomically written.
func New(fsys fsx.FS) (*PIP, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*PIP, error) {
	p := &PIP{
		BasePackageManager: core.NewBasePackageManager(ID, python.ID, tool(goos), manifestFile),
		fsys:               fsys,
		goos:               goos,
	}
	return p, p.declare()
}

// tool describes the interpreter pip is reached through and how to get one.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: shared.SystemPython(goos), VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "python"}
		t.InstallHint = "brew install python"
	case "windows":
		t.InstallHint = "install Python 3 from https://www.python.org/downloads/ (it includes pip)"
	default:
		t.InstallHint = "install Python 3 with your system package manager (it must include venv and pip)"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (p *PIP) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInstall:  p.install,
		core.CapList:     p.list,
		core.CapOutdated: p.outdated,
	}
	filled := map[core.Capability]core.Handler{
		core.CapInit:    p.initProject,
		core.CapAdd:     p.add,
		core.CapRemove:  p.remove,
		core.CapUpdate:  p.update,
		core.CapRun:     p.runScript,
		core.CapBuild:   p.build,
		core.CapTest:    p.test,
		core.CapLint:    p.lint,
		core.CapAudit:   p.audit,
		core.CapPublish: p.publish,
	}
	for _, c := range core.AllCapabilities {
		var err error
		switch {
		case native[c] != nil:
			err = p.Declare(c, core.Native(), native[c])
		case filled[c] != nil:
			err = p.Declare(c, core.Filled(), filled[c])
		default:
			err = p.Declare(c, core.Unsupported("up/down are not available for this manager yet"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Detect implements core.PackageManager: a requirements.txt. Lockfile-owning
// managers outrank it (see DetectRank).
func (p *PIP) Detect(fsys fs.FS) bool { return p.BasePackageManager.Detect(fsys) }

// DetectRank implements core.DetectRanker.
func (p *PIP) DetectRank(fsys fs.FS) int {
	if p.Detect(fsys) {
		return rankRequirements
	}
	return 0
}

// Diagnose implements core.Diagnoser.
func (p *PIP) Diagnose(fsys fs.FS) []core.Finding {
	if !p.Detect(fsys) || shared.HasVenv(fsys) {
		return nil
	}
	return []core.Finding{{
		Severity: core.SeverityWarn,
		Message:  "requirements.txt has no .venv: dependencies are not installed in a project environment",
		Fix:      core.CapInstall,
	}}
}
