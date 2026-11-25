// Package bun is the bun package manager for Node projects. It routes
// capabilities to the bun binary; bun owns package.json and bun.lock, and upc
// changes them through bun (ADR-0002, ADR-0003). bun is used as a package
// manager only: scripts and files still run under node (shared.RuntimeBinary),
// and the test capability runs the project's `test` script, not `bun test`.
package bun

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
	ID     = "bun"
	Binary = "bun"

	lockFile = "bun.lock"

	// legacyLockFile is the binary lock file bun wrote before 1.2.
	legacyLockFile = "bun.lockb"
)

// Detection ranks: a lock file outranks a `packageManager` declaration, which
// outranks the bare manifest that npm also matches.
const (
	rankLockfile = 300
	rankDeclared = 200
)

// Bun implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type Bun struct {
	core.BasePackageManager
	fsys fsx.FS
}

var (
	_ core.PackageManager = (*Bun)(nil)
	_ core.Diagnoser      = (*Bun)(nil)
	_ core.DetectRanker   = (*Bun)(nil)
)

// New builds bun for the running platform. fsys is where new projects and
// script sources are created.
func New(fsys fsx.FS) (*Bun, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*Bun, error) {
	bn := &Bun{
		BasePackageManager: core.NewBasePackageManager(ID, node.ID, tool(goos), lockFile, legacyLockFile, node.PackageFilename),
		fsys:               fsys,
	}
	return bn, bn.declare()
}

// tool describes the binary and how to install it on goos.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: Binary, VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "bun"}
		t.InstallHint = strings.Join(t.InstallCmd, " ")
	default:
		t.InstallHint = "install bun with `npm install --global bun` or from https://bun.sh/docs/installation"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (bn *Bun) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     bn.initProject,
		core.CapInstall:  bn.install,
		core.CapAdd:      bn.add,
		core.CapRemove:   bn.remove,
		core.CapList:     bn.list,
		core.CapUpdate:   bn.update,
		core.CapOutdated: bn.outdated,
		core.CapRun:      bn.runScript,
		core.CapAudit:    bn.audit,
		core.CapPublish:  bn.publish,
		core.CapTest:     bn.test,
	}
	filled := map[core.Capability]core.Handler{
		core.CapBuild: bn.build,
		core.CapLint:  bn.lint,
		core.CapUp:    bn.up,
		core.CapDown:  bn.down,
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = bn.Declare(c, core.Native(), h)
		} else if h, ok := filled[c]; ok {
			err = bn.Declare(c, core.Filled(), h)
		} else {
			err = bn.Declare(c, core.Unsupported("capability is not available for this manager"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DetectRank implements core.DetectRanker: bun.lock 300, a package.json
// `packageManager` of bun@... 200. A bare manifest is npm's to claim.
func (bn *Bun) DetectRank(fsys fs.FS) int {
	switch {
	case hasLock(fsys):
		return rankLockfile
	case declaresBun(fsys):
		return rankDeclared
	}
	return 0
}

func declaresBun(fsys fs.FS) bool {
	p, err := node.ReadProject(fsys)
	return err == nil && strings.HasPrefix(p.PackageManager, ID+"@")
}

// Diagnose implements core.Diagnoser: problems bun itself would trip over.
func (bn *Bun) Diagnose(fsys fs.FS) []core.Finding {
	if !exists(fsys, node.PackageFilename) {
		return nil
	}
	var findings []core.Finding
	if p, err := node.ReadProject(fsys); err == nil && p.PackageManager != "" && !strings.HasPrefix(p.PackageManager, ID+"@") {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  fmt.Sprintf("package.json declares packageManager %q, not bun", p.PackageManager),
		})
	}
	if !hasLock(fsys) {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  "bun.lock is missing, so dependencies are not locked",
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

// requireManifest checks that the directory holds a manifest: bun would
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

// begin starts an invocation of bun in a directory that holds a manifest.
func begin(ctx context.Context, req core.Request) (*invoke.Invocation, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	return invoke.Begin(ctx, req, Binary)
}

// hasLock reports whether the project has a bun lock file, text or legacy binary.
func hasLock(fsys fs.FS) bool { return exists(fsys, lockFile) || exists(fsys, legacyLockFile) }
