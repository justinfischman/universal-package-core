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
