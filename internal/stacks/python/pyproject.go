package python

import (
	"fmt"
	"io/fs"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// PyprojectFilename is the manifest file name.
const PyprojectFilename = "pyproject.toml"

// Project is the read-only subset of pyproject.toml that upc uses.
type Project struct {
	Name           string
	RequiresPython string
	// ProjectScripts is [project.scripts]: console entry points.
	ProjectScripts map[string]string
	// UpcScripts is [tool.upc.scripts]: named commands run through the manager.
	UpcScripts map[string]string
	// HasToolUv reports whether a [tool.uv] table exists.
	HasToolUv bool
	// ToolTables is the set of [tool.*] table names present.
	ToolTables map[string]bool
	// Dependencies is the normalized names of every declared dependency:
	// [project.dependencies], [project.optional-dependencies] and
	// [dependency-groups], plus Poetry's [tool.poetry.dependencies],
	// [tool.poetry.group.*.dependencies] and [tool.poetry.dev-dependencies]
	// (the `python` constraint is not a dependency).
	Dependencies map[string]bool
}

type pyprojectDoc struct {
	Project struct {
		Name           string            `toml:"name"`
		RequiresPython string            `toml:"requires-python"`
		Scripts        map[string]string `toml:"scripts"`
		Dependencies   []string          `toml:"dependencies"`
		Optional       map[string][]any  `toml:"optional-dependencies"`
	} `toml:"project"`
	Groups map[string][]any `toml:"dependency-groups"`
	Tool   map[string]any   `toml:"tool"`
}

// ReadProject parses pyproject.toml. It never writes: the package manager owns
// the file.
func ReadProject(fsys fs.FS) (Project, error) {
	data, err := fs.ReadFile(fsys, PyprojectFilename)
	if err != nil {
		return Project{}, err
	}
	var doc pyprojectDoc
	if err := toml.Unmarshal(data, &doc); err != nil {
		return Project{}, fmt.Errorf("%w: %s: %v", core.ErrInvalidRequest, PyprojectFilename, err)
	}
	p := Project{
		Name:           doc.Project.Name,
		RequiresPython: doc.Project.RequiresPython,
		ProjectScripts: doc.Project.Scripts,
	}
	if _, ok := doc.Tool["uv"]; ok {
		p.HasToolUv = true
	}
	p.ToolTables = make(map[string]bool, len(doc.Tool))
	for name := range doc.Tool {
		p.ToolTables[name] = true
	}
	p.Dependencies = map[string]bool{}
	for _, spec := range doc.Project.Dependencies {
		p.Dependencies[RequirementName(spec)] = true
	}
	for _, group := range []map[string][]any{doc.Project.Optional, doc.Groups} {
		for _, specs := range group {
			for _, v := range specs {
				if spec, ok := v.(string); ok { // other entries are include-group tables
					p.Dependencies[RequirementName(spec)] = true
				}
			}
		}
	}
	addPoetryDependencies(p.Dependencies, doc.Tool["poetry"])
	if upc, ok := doc.Tool["upc"].(map[string]any); ok {
		if raw, ok := upc["scripts"].(map[string]any); ok {
			p.UpcScripts = make(map[string]string, len(raw))
			for k, v := range raw {
				s, ok := v.(string)
				if !ok {
					return Project{}, fmt.Errorf("%w: [tool.upc.scripts] %q must be a string command", core.ErrInvalidRequest, k)
				}
				p.UpcScripts[k] = s
			}
		}
	}
	return p, nil
}

// addPoetryDependencies records the names declared in a [tool.poetry] table.
// Poetry keys dependencies by name, with a version string or table as value.
func addPoetryDependencies(into map[string]bool, poetry any) {
	table, ok := poetry.(map[string]any)
	if !ok {
		return
	}
	add := func(deps any) {
		names, _ := deps.(map[string]any)
		for name := range names {
			if n := NormalizeName(name); n != "python" {
				into[n] = true
			}
		}
	}
	add(table["dependencies"])
	add(table["dev-dependencies"])
	groups, _ := table["group"].(map[string]any)
	for _, g := range groups {
		if group, ok := g.(map[string]any); ok {
			add(group["dependencies"])
		}
	}
}

// Scripts returns the script table: [tool.upc.scripts] commands and
// [project.scripts] entry points.
func (p Project) Scripts() scripts.Table {
	return scripts.Table{Commands: p.UpcScripts, EntryPoints: p.ProjectScripts}
}

// ScriptReader implements scripts.Reader for pyproject.toml.
type ScriptReader struct{}

var _ scripts.Reader = ScriptReader{}

// ReadScripts implements scripts.Reader.
func (ScriptReader) ReadScripts(fsys fs.FS) (scripts.Table, error) {
	p, err := ReadProject(fsys)
	if err != nil {
		return scripts.Table{}, err
	}
	return p.Scripts(), nil
}

// Pyproject implements core.PackageFile.
type Pyproject struct{}

var _ core.PackageFile = Pyproject{}

// Filename implements core.PackageFile.
func (Pyproject) Filename() string { return PyprojectFilename }

// Read implements core.PackageFile. Scripts merges [project.scripts] with
// [tool.upc.scripts]; the latter wins, matching the run lookup order.
func (Pyproject) Read(fsys fs.FS) (core.Manifest, error) {
	p, err := ReadProject(fsys)
	if err != nil {
		return core.Manifest{}, err
	}
	scripts := map[string]string{}
	for k, v := range p.ProjectScripts {
		scripts[k] = v
	}
	for k, v := range p.UpcScripts {
		scripts[k] = v
	}
	return core.Manifest{Name: p.Name, Scripts: scripts}, nil
}
