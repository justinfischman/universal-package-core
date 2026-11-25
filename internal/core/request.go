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
	// Groups are further dependency groups to install together with Group,
	// as selected by an environment (ADR-0015). Only install reads them.
	Groups []string
	// Script is the script to run (run), or the standalone script file the
	// dependency commands operate on (add, remove, install, ...). For init in
	// script mode it is the name of the script to create.
	Script string
	// ProjectType is the project type being scaffolded (init), e.g. "script".
	ProjectType string
	// Kind is the app sub-type ("cli" or "library") when init scaffolds an app;
	// empty is the default service.
	Kind string
	// Name is the project to create when init scaffolds a project directory
	// (for example "package"): the manager creates Dir/Name.
	Name string
	// Version is a neutral version constraint (">=2.3.4") for add; see
	// ParseConstraint. Only managers implementing Versioned accept it.
	Version string
	// Member is the workspace member to act on (if any).
	Member string
	// Env is extra KEY=VALUE child environment, already resolved. Managers
	// that start processes append it to the child's environment.
	Env    []string
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

// Versioned is implemented by managers that render a Request.Version into their
// add command. Managers that do not implement it must never see a Version.
type Versioned interface {
	ConstraintDialect() Dialect
}

// RenderConstraint parses Version and renders it for the manager's dialect.
// It returns "" when no version was requested. A version applies to exactly one
// package, and that package must not already carry its own constraint.
func (r Request) RenderConstraint(d Dialect, manager string) (string, error) {
	if r.Version == "" {
		return "", nil
	}
	if len(r.Packages) != 1 {
		return "", fmt.Errorf("%w: a version constraint applies to exactly one package, got %d", ErrInvalidRequest, len(r.Packages))
	}
	p := r.Packages[0]
	if strings.ContainsAny(p, "<>=!~^,; ") || strings.Contains(strings.TrimPrefix(p, "@"), "@") {
		return "", fmt.Errorf("%w: package %q already carries a version; drop it or the version flag", ErrInvalidRequest, p)
	}
	c, err := ParseConstraint(r.Version)
	if err != nil {
		return "", err
	}
	return c.Render(d, manager)
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

// AllGroups returns Group followed by Groups, without empty names or
// duplicates, in the order given.
func (r Request) AllGroups() []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range append([]string{r.Group}, r.Groups...) {
		if g != "" && !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return out
}
