package node

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// PackageFilename is the manifest file name.
const PackageFilename = "package.json"

// Project is the read-only subset of package.json that upc uses.
type Project struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Scripts map[string]string `json:"scripts"`
	Engines map[string]string `json:"engines"`
	// PackageManager is the `packageManager` field, e.g. "npm@11.0.0".
	PackageManager string `json:"packageManager"`
}

// ReadProject parses package.json. It never writes: the package manager owns
// the file.
func ReadProject(fsys fs.FS) (Project, error) {
	data, err := fs.ReadFile(fsys, PackageFilename)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return Project{}, fmt.Errorf("%w: %s: %v", core.ErrInvalidRequest, PackageFilename, err)
	}
	return p, nil
}

// ScriptReader implements scripts.Reader for package.json: its `scripts` are
// command lines the package manager runs by name.
type ScriptReader struct{}

var _ scripts.Reader = ScriptReader{}

// ReadScripts implements scripts.Reader.
func (ScriptReader) ReadScripts(fsys fs.FS) (scripts.Table, error) {
	p, err := ReadProject(fsys)
	if err != nil {
		return scripts.Table{}, err
	}
	return scripts.Table{Commands: p.Scripts}, nil
}

// Package implements core.PackageFile.
type Package struct{}

var _ core.PackageFile = Package{}

// Filename implements core.PackageFile.
func (Package) Filename() string { return PackageFilename }

// Read implements core.PackageFile.
func (Package) Read(fsys fs.FS) (core.Manifest, error) {
	p, err := ReadProject(fsys)
	if err != nil {
		return core.Manifest{}, err
	}
	return core.Manifest{Name: p.Name, Scripts: p.Scripts}, nil
}
