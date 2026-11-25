// Package pnpm is the pnpm package manager for Node. It routes capabilities to
// the pnpm binary; pnpm owns package.json and pnpm-lock.yaml, and upc changes
// them through pnpm (ADR-0002, ADR-0003).
package pnpm

import (
	"context"
	"fmt"
	"io/fs"
	"runtime"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
)

// ID is the package manager id; Binary is the executable it wraps.
const (
	ID     = "pnpm"
	Binary = "pnpm"

	lockFile = "pnpm-lock.yaml"
)

// Detection ranks: a lock file outranks a `packageManager` declaration, which
// outranks the bare manifest that npm also matches.
const (
	rankLockfile = 300
	rankDeclared = 200
)

// Pnpm implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type Pnpm struct {
	core.BasePackageManager
	fsys fsx.FS
}

var (
	_ core.PackageManager = (*Pnpm)(nil)
	_ core.Diagnoser      = (*Pnpm)(nil)
	_ core.DetectRanker   = (*Pnpm)(nil)
)

// New builds pnpm for the running platform. fsys is where new projects and
// script sources are created.
func New(fsys fsx.FS) (*Pnpm, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*Pnpm, error) {
	pn := &Pnpm{
		BasePackageManager: core.NewBasePackageManager(ID, node.ID, tool(goos), lockFile, node.PackageFilename),
		fsys:               fsys,
	}
	return pn, pn.declare()
}

// tool describes the binary and how to install it on goos.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: Binary, VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "pnpm"}
		t.InstallHint = strings.Join(t.InstallCmd, " ")
	default:
		t.InstallHint = "install pnpm with `corepack enable` or `npm install --global pnpm` (see https://pnpm.io/installation)"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (pn *Pnpm) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     pn.initProject,
		core.CapInstall:  pn.install,
		core.CapAdd:      pn.add,
		core.CapRemove:   pn.remove,
		core.CapList:     pn.list,
		core.CapUpdate:   pn.update,
		core.CapOutdated: pn.outdated,
		core.CapRun:      pn.runScript,
		core.CapAudit:    pn.audit,
		core.CapPublish:  pn.publish,
		core.CapTest:     pn.test,
	}
	filled := map[core.Capability]core.Handler{
		core.CapBuild: pn.build,
		core.CapLint:  pn.lint,
		core.CapUp:    pn.up,
		core.CapDown:  pn.down,
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = pn.Declare(c, core.Native(), h)
		} else if h, ok := filled[c]; ok {
			err = pn.Declare(c, core.Filled(), h)
		} else {
			err = pn.Declare(c, core.Unsupported("capability is not available for this manager"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DetectRank implements core.DetectRanker: pnpm-lock.yaml 300, a package.json
// `packageManager` of pnpm@... 200. A bare manifest is npm's to claim.
func (pn *Pnpm) DetectRank(fsys fs.FS) int {
	switch {
	case exists(fsys, lockFile):
		return rankLockfile
	case declaresPnpm(fsys):
		return rankDeclared
	}
	return 0
}

func declaresPnpm(fsys fs.FS) bool {
	p, err := node.ReadProject(fsys)
	return err == nil && strings.HasPrefix(p.PackageManager, ID+"@")
}

// Diagnose implements core.Diagnoser: problems pnpm itself would trip over.
func (pn *Pnpm) Diagnose(fsys fs.FS) []core.Finding {
	if !exists(fsys, node.PackageFilename) {
		return nil
	}
	var findings []core.Finding
	if p, err := node.ReadProject(fsys); err == nil && p.PackageManager != "" && !strings.HasPrefix(p.PackageManager, ID+"@") {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  fmt.Sprintf("package.json declares packageManager %q, not pnpm", p.PackageManager),
		})
	}
	if !exists(fsys, lockFile) {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  "pnpm-lock.yaml is missing, so dependencies are not locked",
			Fix:      core.CapInstall,
		})
	} else {
		findings = append(findings, lockDrift(fsys)...)
	}
	return findings
}

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

// readProject reads the directory's package.json, refusing a directory without one.
func readProject(req core.Request) (node.Project, error) {
	if err := requireManifest(req); err != nil {
		return node.Project{}, err
	}
	return node.ReadProject(invoke.DirFS(req.Dir))
}

// requireManifest checks that the directory holds a manifest: pnpm would
// otherwise walk up to a parent project or fail obscurely.
func requireManifest(req core.Request) error {
	if exists(invoke.DirFS(req.Dir), node.PackageFilename) {
		return nil
	}
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	return fmt.Errorf("%w: no package.json in %s; run `upc init` or `upc new`", core.ErrInvalidRequest, dir)
}

// begin starts an invocation of pnpm in a directory that holds a manifest.
func begin(ctx context.Context, req core.Request) (*invoke.Invocation, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	return invoke.Begin(ctx, req, Binary)
}
