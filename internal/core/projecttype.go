package core

import "context"

// ScaffoldRequest asks a project type to create a new project.
type ScaffoldRequest struct {
	Request
	Name           string
	Language       string
	PackageManager string
}

// ProjectType is a kind of project (script, package, app).
type ProjectType interface {
	Module
	// Supports reports whether it can scaffold the language/manager pair.
	Supports(language, packageManager string) bool
	// Scaffold creates the project, preferring the manager's native scaffolding.
	Scaffold(ctx context.Context, req ScaffoldRequest) (Result, error)
}
