// Package node is the Node language module. It is the only place (with its
// package manager subpackages) that knows Node file conventions (ADR-0002).
// TypeScript is a template and tooling variant of this language, not a
// separate language (ADR-0010).
package node

import (
	"embed"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ID is the language id.
const ID = "node"

// markers whose presence in a directory means a Node project.
var markers = []string{"package.json", "tsconfig.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock", "bun.lockb"}

// TSConfigFilename is the TypeScript compiler configuration file name.
const TSConfigFilename = "tsconfig.json"

//go:embed all:templates
var templates embed.FS

// Language implements core.Language and core.Templater.
type Language struct{}

var (
	_ core.Language  = Language{}
	_ core.Templater = Language{}
)

// New returns the Node language module.
func New() Language { return Language{} }

// ID implements core.Module.
func (Language) ID() string { return ID }

// Kind implements core.Module.
func (Language) Kind() core.Kind { return core.KindLanguage }

// PackageManagers implements core.Language.
func (Language) PackageManagers() []string { return []string{"npm", "yarn", "pnpm", "bun"} }

// DefaultPackageManager implements core.Language.
func (Language) DefaultPackageManager() string { return "npm" }

// Detect implements core.Language: a manifest, a TypeScript configuration or a
// lock file.
func (Language) Detect(fsys fs.FS) bool {
	for _, m := range markers {
		if _, err := fs.Stat(fsys, m); err == nil {
			return true
		}
	}
	return false
}

// IsTypeScript reports whether the directory is configured for TypeScript.
func IsTypeScript(fsys fs.FS) bool {
	_, err := fs.Stat(fsys, TSConfigFilename)
	return err == nil
}

// Templates implements core.Templater: files under templates/<projectType>.
func (Language) Templates(projectType string) (fs.FS, bool) {
	sub, err := fs.Sub(templates, "templates/"+projectType)
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "."); err != nil {
		return nil, false
	}
	return sub, true
}
