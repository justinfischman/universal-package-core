package python

import (
	"fmt"
	"io/fs"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// GroupDecls is the set of dependency-group names pyproject.toml declares, by
// the table that declares them.
type GroupDecls struct {
	// DependencyGroups is [dependency-groups] (PEP 735).
	DependencyGroups map[string]bool
	// Optional is [project.optional-dependencies] (extras).
	Optional map[string]bool
	// Poetry is [tool.poetry.group.<name>], plus "dev" when the legacy
	// [tool.poetry.dev-dependencies] table exists.
	Poetry map[string]bool
}

type groupDeclDoc struct {
	Project struct {
		Optional map[string]any `toml:"optional-dependencies"`
	} `toml:"project"`
	Groups map[string]any `toml:"dependency-groups"`
	Tool   struct {
		Poetry struct {
			Group   map[string]any `toml:"group"`
			DevDeps map[string]any `toml:"dev-dependencies"`
		} `toml:"poetry"`
	} `toml:"tool"`
}

// ReadGroupDecls parses the dependency-group names of pyproject.toml. It never
// writes: the package manager owns the file.
func ReadGroupDecls(fsys fs.FS) (GroupDecls, error) {
	data, err := fs.ReadFile(fsys, PyprojectFilename)
	if err != nil {
		return GroupDecls{}, err
	}
	var doc groupDeclDoc
	if err := toml.Unmarshal(data, &doc); err != nil {
		return GroupDecls{}, fmt.Errorf("%w: %s: %v", core.ErrInvalidRequest, PyprojectFilename, err)
	}
	d := GroupDecls{
		DependencyGroups: keySet(doc.Groups),
		Optional:         keySet(doc.Project.Optional),
		Poetry:           keySet(doc.Tool.Poetry.Group),
	}
	if len(doc.Tool.Poetry.DevDeps) > 0 {
		d.Poetry["dev"] = true
	}
	return d, nil
}

func keySet(m map[string]any) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[NormalizeName(k)] = true
	}
	return out
}
