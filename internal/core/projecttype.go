package core

import (
	"context"
	"io/fs"
)

// ScaffoldRequest asks a project type to create a new project.
type ScaffoldRequest struct {
	Request
	Name           string
	Language       string
	PackageManager string
	// Kind is the project type's sub-type ("cli", "library"); empty is the
	// type's default. Only a type that declares sub-types accepts one.
	Kind string
	// Manager is the chosen package manager; the project type asks it to
	// initialise the project natively instead of naming any concrete one.
	Manager PackageManager
	// Templates holds the language's files for this project type, if any.
	Templates fs.FS
}

// ProjectType is a kind of project (script, package, app).
type ProjectType interface {
	Module
	// Supports reports whether it can scaffold the language/manager pair.
	Supports(language, packageManager string) bool
	// Scaffold creates the project, preferring the manager's native scaffolding.
	Scaffold(ctx context.Context, req ScaffoldRequest) (Result, error)
}
