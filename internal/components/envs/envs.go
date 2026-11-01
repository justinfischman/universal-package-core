// Package envs finds the environments a project declares or implies (ADR-0006).
// It lists names and file paths only. It never opens an env file, so values and
// secrets never reach upc.
package envs

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Where an environment came from.
const (
	SourceConfig     = "config"
	SourceConvention = "convention"
)

// conventionPrefix names convention files: `.env.<name>`.
const conventionPrefix = ".env."

// templates are checked-in examples, not environments.
var templates = map[string]bool{"example": true, "sample": true, "template": true, "dist": true}

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Entry is one environment.
type Entry struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Source string `json:"source"`
	// Exists is whether the file is present in the project.
	Exists bool `json:"exists"`
}

// ValidName rejects names that are not plain words, so a name is safe to put in
// a file name and in UPC_ENV.
func ValidName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid environment name %q (use letters, digits, '-' and '_')", name)
	}
	return nil
}

// ValidFile rejects absolute paths and paths that leave the project.
func ValidFile(file string) error {
	clean := path.Clean(file)
	if file == "" || path.IsAbs(file) || strings.Contains(file, `\`) || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("env file %q must be a relative path inside the project", file)
	}
	return nil
}

// Discover returns the declared environments (name to file; an empty file means
// the convention file) followed by `.env.<name>` files in the project root that
// no declaration covers. The result is sorted by name. root is the project dir.
func Discover(root fs.FS, declared map[string]string) ([]Entry, error) {
	byName := map[string]Entry{}
	for name, file := range declared {
		if file == "" {
			file = conventionPrefix + name
		}
		byName[name] = Entry{Name: name, File: file, Source: SourceConfig, Exists: exists(root, file)}
	}
	items, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil, fmt.Errorf("read project directory: %w", err)
	}
	for _, it := range items {
		name, ok := strings.CutPrefix(it.Name(), conventionPrefix)
		if !ok || it.IsDir() || templates[name] || ValidName(name) != nil {
			continue
		}
		if _, declared := byName[name]; !declared {
			byName[name] = Entry{Name: name, File: it.Name(), Source: SourceConvention, Exists: true}
		}
	}
	out := make([]Entry, 0, len(byName))
	for _, e := range byName {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Find returns the named environment.
func Find(entries []Entry, name string) (Entry, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Check reports why name cannot be used: not declared or discovered, or its file
// is missing. It returns nil for a usable environment.
func Check(entries []Entry, name string) error {
	e, ok := Find(entries, name)
	switch {
	case !ok:
		return fmt.Errorf("environment %q does not exist (declare [environments.%s] in the project config or add %s%s)", name, name, conventionPrefix, name)
	case !e.Exists:
		return fmt.Errorf("environment %q points at %s, which does not exist", name, e.File)
	}
	return nil
}

// exists reports whether file is a regular file; it never reads it.
func exists(root fs.FS, file string) bool {
	info, err := fs.Stat(root, path.Clean(file))
	return err == nil && info.Mode().IsRegular()
}
