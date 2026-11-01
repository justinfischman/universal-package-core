// Package python is the Python language module. It is the only place (with
// its package manager subpackages) that knows Python file conventions (ADR-0002).
package python

import (
	"embed"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ID is the language id.
const ID = "python"

// Markers whose presence in a directory means a Python project.
var markers = []string{"pyproject.toml", "requirements.txt", "setup.py"}

//go:embed all:templates
var templates embed.FS

// Language implements core.Language and core.Templater.
type Language struct{}

var (
	_ core.Language  = Language{}
	_ core.Templater = Language{}
)

// New returns the Python language module.
func New() Language { return Language{} }

// ID implements core.Module.
func (Language) ID() string { return ID }

// Kind implements core.Module.
func (Language) Kind() core.Kind { return core.KindLanguage }

// PackageManagers implements core.Language.
func (Language) PackageManagers() []string { return []string{"uv", "pip", "poetry"} }

// DefaultPackageManager implements core.Language.
func (Language) DefaultPackageManager() string { return "uv" }

// Detect implements core.Language: a marker file, or a *.py file carrying a
// PEP 723 inline metadata block.
func (Language) Detect(fsys fs.FS) bool {
	for _, m := range markers {
		if _, err := fs.Stat(fsys, m); err == nil {
			return true
		}
	}
	scripts, err := ScriptFiles(fsys)
	return err == nil && len(scripts) > 0
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
