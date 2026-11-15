// Package npm is the npm package manager for Node. It routes capabilities to
// the npm binary; npm owns package.json and package-lock.json, and upc only
// changes them through npm (ADR-0002, ADR-0003).
package npm

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
	ID     = "npm"
	Binary = "npm"

	lockFile = "package-lock.json"
)

// Detection ranks: a lock file outranks a bare manifest (which other managers
// of the language share).
const (
	rankLockfile = 300
	rankManifest = 100
)

// NPM implements core.PackageManager, core.Diagnoser and core.DetectRanker.
type NPM struct {
	core.BasePackageManager
	fsys fsx.FS
}

var (
	_ core.PackageManager = (*NPM)(nil)
	_ core.Diagnoser      = (*NPM)(nil)
	_ core.DetectRanker   = (*NPM)(nil)
)

// New builds npm for the running platform. fsys is where new projects and
// script sources are created.
func New(fsys fsx.FS) (*NPM, error) { return newFor(fsys, runtime.GOOS) }

func newFor(fsys fsx.FS, goos string) (*NPM, error) {
	n := &NPM{
		BasePackageManager: core.NewBasePackageManager(ID, node.ID, tool(goos), lockFile, node.PackageFilename),
		fsys:               fsys,
	}
	return n, n.declare()
}

// tool describes the binary and how to install it on goos. npm ships with Node.
func tool(goos string) core.Tool {
	t := core.Tool{Binary: Binary, VersionArgs: []string{"--version"}}
	switch goos {
	case "darwin":
		t.InstallCmd = []string{"brew", "install", "node"}
		t.InstallHint = strings.Join(t.InstallCmd, " ")
	case "windows":
		t.InstallHint = "install Node.js from https://nodejs.org/ (it includes npm)"
	default:
		t.InstallHint = "install Node.js with your system package manager or from https://nodejs.org/ (it includes npm)"
	}
	return t
}

// declare records every capability (ADR-0003 parity).
func (n *NPM) declare() error {
	native := map[core.Capability]core.Handler{
		core.CapInit:     n.initProject,
		core.CapInstall:  n.install,
		core.CapAdd:      n.add,
		core.CapRemove:   n.remove,
		core.CapList:     n.list,
		core.CapUpdate:   n.update,
		core.CapOutdated: n.outdated,
		core.CapRun:      n.runScript,
		core.CapAudit:    n.audit,
		core.CapPublish:  n.publish,
		core.CapTest:     n.test,
	}
	filled := map[core.Capability]core.Handler{
		core.CapBuild: n.build,
		core.CapLint:  n.lint,
		core.CapUp:    n.up,
		core.CapDown:  n.down,
	}
	for _, c := range core.AllCapabilities {
		var err error
		if h, ok := native[c]; ok {
			err = n.Declare(c, core.Native(), h)
		} else if h, ok := filled[c]; ok {
			err = n.Declare(c, core.Filled(), h)
		} else {
			err = n.Declare(c, core.Unsupported("capability is not available for this manager"), nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DetectRank implements core.DetectRanker.
func (n *NPM) DetectRank(fsys fs.FS) int {
	switch {
	case !n.Detect(fsys):
		return 0
	case exists(fsys, lockFile):
		return rankLockfile
	}
	return rankManifest
}

// Diagnose implements core.Diagnoser: problems npm itself would trip over.
func (n *NPM) Diagnose(fsys fs.FS) []core.Finding {
	if !exists(fsys, node.PackageFilename) {
		return nil
	}
	var findings []core.Finding
	if p, err := node.ReadProject(fsys); err == nil && p.PackageManager != "" && !strings.HasPrefix(p.PackageManager, ID+"@") {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  fmt.Sprintf("package.json declares packageManager %q, not npm", p.PackageManager),
		})
	}
	if !exists(fsys, lockFile) {
		findings = append(findings, core.Finding{
			Severity: core.SeverityWarn,
			Message:  "package-lock.json is missing, so dependencies are not locked",
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

// requireManifest checks that the directory holds a manifest: npm would otherwise
// create one silently in the wrong place.
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

// begin starts an invocation of npm in a directory that holds a manifest.
func begin(ctx context.Context, req core.Request) (*shared.Invocation, error) {
	if err := requireManifest(req); err != nil {
		return nil, err
	}
	return shared.Begin(ctx, req, Binary)
}
