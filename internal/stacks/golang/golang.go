// Package golang is a SPIKE of a Go language module (ADR-0019). It is not
// registered and has no CLI exposure; it exists to prove that core.Language and
// core.PackageManager fit a language with no scripts, groups or lock file.
package golang

import (
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ID is the language id; ManifestFilename is the module file that marks a project.
const (
	ID               = "go"
	ManifestFilename = "go.mod"
)

// Language implements core.Language. It has no Templater: the spike scaffolds
// nothing, so core.Templater stays optional exactly as designed.
type Language struct{}

var _ core.Language = Language{}

// New returns the Go language module.
func New() Language { return Language{} }

// ID implements core.Module.
func (Language) ID() string { return ID }

// Kind implements core.Module.
func (Language) Kind() core.Kind { return core.KindLanguage }

// PackageManagers implements core.Language.
func (Language) PackageManagers() []string { return []string{"gomod"} }

// DefaultPackageManager implements core.Language.
func (Language) DefaultPackageManager() string { return "gomod" }

// Detect implements core.Language: a go.mod, or a go.work for a multi-module workspace.
func (Language) Detect(fsys fs.FS) bool {
	for _, m := range []string{ManifestFilename, "go.work"} {
		if _, err := fs.Stat(fsys, m); err == nil {
			return true
		}
	}
	return false
}
