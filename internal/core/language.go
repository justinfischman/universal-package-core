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
