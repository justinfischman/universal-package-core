// Package yarn is the yarn package manager for Node. It routes capabilities to
// the yarn binary; yarn owns package.json and yarn.lock, and upc changes them
// through yarn (ADR-0002, ADR-0003). It targets yarn classic (1.x); berry
// (2+), found with `yarn --version`, differs only in the argv noted in
// version.go (ADR-0010 addendum).
package yarn

import (
	"context"
	"fmt"
	"io/fs"
	"runtime"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// ID is the package manager id; Binary is the executable it wraps.
const (
	ID     = "yarn"
	Binary = "yarn"

	lockFile = "yarn.lock"
)

// Detection ranks: a lock file outranks a `packageManager` declaration, which
// outranks the bare manifest that npm also matches.
const (
	rankLockfile = 300
	rankDeclared = 200
)

// Yarn implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type Yarn struct {
	core.BasePackageManager
	fsys fsx.FS
}

var (
	_ core.PackageManager = (*Yarn)(nil)
	_ core.Diagnoser      = (*Yarn)(nil)
	_ core.DetectRanker   = (*Yarn)(nil)
)

// New builds yarn for the running platform. fsys is where new projects and
// script sources are created.
func New(fsys fsx.FS) (*Yarn, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*Yarn, error) {
	y := &Yarn{
		BasePackageManager: core.NewBasePackageManager(ID, node.ID, tool(goos), lockFile, node.PackageFilename),
		fsys:               fsys,
	}
	return y, y.declare()
}

// tool describes the binary and how to install it on goos.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: Binary, VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "yarn"}
		t.InstallHint = strings.Join(t.InstallCmd, " ")
	default:
		t.InstallHint = "install yarn with `corepack enable` or `npm install --global yarn` (see https://yarnpkg.com/getting-started/install)"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (y *Yarn) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     y.initProject,
		core.CapInstall:  y.install,
		core.CapAdd:      y.add,
		core.CapRemove:   y.remove,
		core.CapList:     y.list,
		core.CapUpdate:   y.update,
		core.CapOutdated: y.outdated,
		core.CapRun:      y.runScript,
		core.CapAudit:    y.audit,
		core.CapPublish:  y.publish,
		core.CapTest:     y.test,
	}
	filled := map[core.Capability]core.Handler{
		core.CapBuild: y.build,
		core.CapLint:  y.lint,
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = y.Declare(c, core.Native(), h)
		} else if h, ok := filled[c]; ok {
			err = y.Declare(c, core.Filled(), h)
		} else {
			err = y.Declare(c, core.Unsupported("up/down are not available for this manager yet"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DetectRank implements core.DetectRanker: yarn.lock 300, a package.json
// `packageManager` of yarn@... 200. A bare manifest is npm's to claim.
func (y *Yarn) DetectRank(fsys fs.FS) int {
	switch {
	case exists(fsys, lockFile):
		return rankLockfile
	case declaresYarn(fsys):
		return rankDeclared
	}
	return 0
}

func declaresYarn(fsys fs.FS) bool {
	p, err := node.ReadProject(fsys)
	return err == nil && strings.HasPrefix(p.PackageManager, ID+"@")
}

// Diagnose implements core.Diagnoser: problems yarn itself would trip over.
func (y *Yarn) Diagnose(fsys fs.FS) []core.Finding {
	if !exists(fsys, node.PackageFilename) {
		return nil
	}
	var findings []core.Finding
	if p, err := node.ReadProject(fsys); err == nil && p.PackageManager != "" && !strings.HasPrefix(p.PackageManager, ID+"@") {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  fmt.Sprintf("package.json declares packageManager %q, not yarn", p.PackageManager),
		})
	}
	if !exists(fsys, lockFile) {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  "yarn.lock is missing, so dependencies are not locked",
			Fix:      core.CapInstall,
		})
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
	return node.ReadProject(shared.DirFS(req.Dir))
}

// requireManifest checks that the directory holds a manifest: yarn would
// otherwise walk up to a parent project or fail obscurely.
func requireManifest(req core.Request) error {
	if exists(shared.DirFS(req.Dir), node.PackageFilename) {
		return nil
	}
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	return fmt.Errorf("%w: no package.json in %s; run `upc init` or `upc new`", core.ErrInvalidRequest, dir)
}

// begin starts an invocation of yarn in a directory that holds a manifest.
func begin(ctx context.Context, req core.Request) (*shared.Invocation, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	return shared.Begin(ctx, req, Binary)
}
