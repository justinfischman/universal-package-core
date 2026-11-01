package core

import "io/fs"

// Manifest is the read-only subset of a package manager manifest that upc needs.
type Manifest struct {
	Name    string
	Scripts map[string]string
}

// PackageFile reads a manager's manifest (pyproject.toml, package.json). upc
// reads it to detect and report; the package manager owns writing it.
type PackageFile interface {
	// Filename is the manifest file name.
	Filename() string
	Read(fsys fs.FS) (Manifest, error)
}
