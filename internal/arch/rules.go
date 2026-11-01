// Package arch enforces ADR-0002 (isolation and explicit registration). The
// rules are pure functions over package import data so they can be unit-tested
// with synthetic violations; arch_test.go runs them on the real module.
package arch

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ModulePath is this module's import path prefix.
const ModulePath = "github.com/justinfischman/universal-package-core/"

// Pkg is the import data of one package, as printed by `go list -json`.
type Pkg struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func (p Pkg) allImports() []string {
	all := append([]string{}, p.Imports...)
	all = append(all, p.TestImports...)
	return append(all, p.XTestImports...)
}

// rel trims the module prefix; ok is false for packages outside the module.
func rel(importPath string) (string, bool) {
	if importPath == strings.TrimSuffix(ModulePath, "/") {
		return "", true
	}
	return strings.CutPrefix(importPath, ModulePath)
}

// concreteRoot returns the owning tree ("internal/stacks/python",
// "internal/projects/script") of a concrete package, or "" if it is not one.
func concreteRoot(rel string) string {
	parts := strings.Split(rel, "/")
	if len(parts) >= 3 && parts[0] == "internal" && (parts[1] == "stacks" || parts[1] == "projects") {
		return strings.Join(parts[:3], "/")
	}
	return ""
}

func under(rel, dir string) bool { return rel == dir || strings.HasPrefix(rel, dir+"/") }

// CheckImports returns one message per violated import rule.
func CheckImports(pkgs []Pkg) []string {
	var out []string
	for _, p := range pkgs {
		from, ok := rel(p.ImportPath)
		if !ok {
			continue
		}
		fromRoot := concreteRoot(from)
		for _, imp := range p.allImports() {
			to, ok := rel(imp)
			if !ok {
				continue // standard library or third party
			}
			toRoot := concreteRoot(to)
			switch {
			case under(from, "internal/core"):
				// Rule 1: core imports no concrete or wiring package.
				if !under(to, "internal/core") && !under(to, "internal/components") {
					out = append(out, fmt.Sprintf("core package %s imports %s; core may import only core and components", from, to))
				}
			case fromRoot != "":
				// Rules 3 and 4: concrete trees never import each other, wiring, or the CLI.
				if toRoot != "" && toRoot != fromRoot {
					out = append(out, fmt.Sprintf("%s imports %s; concrete modules must not import each other", from, to))
				}
				if under(to, "internal/register") || under(to, "internal/cli") {
					out = append(out, fmt.Sprintf("%s imports %s; concrete modules must not import wiring or the CLI", from, to))
				}
			default:
				// Only the composition root imports concrete trees.
				if toRoot != "" && !under(from, "internal/register") {
					out = append(out, fmt.Sprintf("%s imports concrete module %s; only internal/register may", from, to))
				}
				// Only cmd/ composes the registry. internal/arch is test-only and reads
				// the registered ids to build its word list.
				if under(to, "internal/register") && !under(from, "cmd") && !under(from, "internal/register") && !under(from, "internal/arch") {
					out = append(out, fmt.Sprintf("%s imports internal/register; only cmd/ may", from))
				}
			}
		}
	}
	return out
}

// WordRule scans text for one module id outside the trees allowed to name it.
type WordRule struct {
	Word string
}

var skipDirs = map[string]bool{".git": true, "bin": true, "dist": true, "target": true, "testdata": true, "plans": true, "docs": true}

// allowedFor reports whether path (slash-separated, relative to the root) may
// mention the module id: its own tree, the composition root, and the
// documentation and spec files.
func allowedFor(path, id string) bool {
	if under(path, "internal/register") || under(path, "internal/arch") {
		return true
	}
	switch filepath.Base(path) {
	case "go.mod", "go.sum":
		return true
	}
	if strings.HasSuffix(path, ".devlang") || strings.HasSuffix(path, ".md") {
		return true
	}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
		if seg == id && (strings.Contains(path, "internal/stacks/") || strings.Contains(path, "internal/projects/")) {
			return true
		}
	}
	return false
}

// CheckWords walks root and reports every file that names a module id outside
// its allowed trees. Matching is case-insensitive and whole-word.
func CheckWords(root string, ids []string) ([]string, error) {
	res := make(map[string]*regexp.Regexp, len(ids))
	for _, id := range ids {
		res[id] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(id) + `\b`)
	}
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, ".tmpl") && !strings.HasSuffix(p, ".toml") {
			return nil
		}
		relPath, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if allowedFor(relPath, id) {
				continue
			}
			if res[id].Match(data) {
				out = append(out, fmt.Sprintf("%s mentions %q outside its own tree", relPath, id))
			}
		}
		return nil
	})
	return out, err
}
