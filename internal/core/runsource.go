package core

import "context"

// RunSource is a tool-neutral place runnable recipes come from (ADR-0016). It
// ranks below the manifest sources in the run lookup. A source that cannot
// serve a project (no recipe file, tool not installed) reports no recipes and
// no error, so the project behaves as if the source did not exist.
type RunSource interface {
	// Recipes lists the recipe names runnable in dir; nil when none or when
	// the source is unavailable.
	Recipes(ctx context.Context, dir string) ([]string, error)
	// Run runs one recipe in dir with extra arguments, streaming its output.
	Run(ctx context.Context, dir, recipe string, args []string) error
}

// RecipeFile is a recipe file a RunSource generates for a project.
type RecipeFile struct {
	// Name is the file name, relative to the project directory.
	Name string
	// Body is the file content.
	Body string
}

// RecipeExporter is a RunSource that can also write the project's upc
// capabilities as recipes of its own format.
type RecipeExporter interface {
	// Exists reports whether dir already holds the source's recipe file.
	Exists(dir string) bool
	// RecipeFile renders one recipe per verb that calls program with that verb;
	// the recipe's own command line supplies extra arguments.
	RecipeFile(program string, verbs []string) RecipeFile
}

// UpTarget identifies the project to bring up or down. App isolates one app
// of a workspace from the others (ADR-0014); it may be empty for a single
// project.
type UpTarget struct {
	Dir string
	App string
}

// UpProvider brings a project up and down (ADR-0016). It is consulted after
// the configured up command and the dev/start scripts.
type UpProvider interface {
	// Name is the stable id recorded in run state so down reaches the
	// provider that started the project.
	Name() string
	// Detect reports whether the provider can serve the project in dir.
	Detect(dir string) bool
	// Up starts the project.
	Up(ctx context.Context, target UpTarget) error
	// Down stops what Up started.
	Down(ctx context.Context, target UpTarget) error
}
