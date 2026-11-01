package core

import (
	"fmt"
	"strings"
)

// Request is the single input shape for every capability (ADR-0003).
type Request struct {
	Dir      string   // project root
	Packages []string // package specs for add/remove
	Group    string   // dependency group, e.g. "dev"
	// Script is the script to run (run), or the standalone script file the
	// dependency commands operate on (add, remove, install, ...). For init in
	// script mode it is the name of the script to create.
	Script string
	// ProjectType is the project type being scaffolded (init), e.g. "script".
	ProjectType string
	// Name is the project to create when init scaffolds a project directory
	// (for example "package"): the manager creates Dir/Name.
	Name   string
	Args   []string // passthrough arguments
	DryRun bool
	Yes    bool
}

// Result is the single output shape for every capability.
type Result struct {
	Summary  string   `json:"summary"`
	Data     any      `json:"data,omitempty"`
	Commands []string `json:"commands,omitempty"`
}

// Package is one dependency as reported by list and outdated.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Latest  string `json:"latest,omitempty"`
}

// RequirePackages checks Packages for add/remove: at least one, each a
// non-empty spec that cannot be mistaken for a command-line flag.
func (r Request) RequirePackages() error {
	if len(r.Packages) == 0 {
		return fmt.Errorf("%w: at least one package is required", ErrInvalidRequest)
	}
	return r.ValidatePackages()
}

// ValidatePackages rejects empty specs and specs that start with "-".
func (r Request) ValidatePackages() error {
	for _, p := range r.Packages {
		if strings.TrimSpace(p) == "" || strings.HasPrefix(p, "-") {
			return fmt.Errorf("%w: %q is not a valid package spec", ErrInvalidRequest, p)
		}
	}
	return nil
}
