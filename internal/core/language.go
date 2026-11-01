package core

import "io/fs"

// Language is a programming language upc can set up and manage.
type Language interface {
	Module
	// PackageManagers lists the ids of its managers.
	PackageManagers() []string
	// DefaultPackageManager is the id used when none is chosen.
	DefaultPackageManager() string
	// Detect reports whether the directory contains this language.
	Detect(fsys fs.FS) bool
}

// Templater is optionally implemented by a Language that ships files for a
// project type (ADR-0005).
type Templater interface {
	// Templates returns the embedded files for a project type id.
	Templates(projectType string) (fs.FS, bool)
}
