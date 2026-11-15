package shared

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
)

// AppServerCommand is the command the app template's server runs with; the
// template file is templates/app/server.py of the python language.
const AppServerCommand = "python server.py"

// AppendAppScripts adds the dev and start commands to the [tool.upc.scripts]
// table of the pyproject.toml in projectDir, so `upc up` can start the app.
// It is for a freshly scaffolded project: an existing [tool.upc] table is
// refused rather than merged. All writes go through fsys.
func AppendAppScripts(fsys fsx.FS, projectDir string) error {
	path := filepath.Join(projectDir, "pyproject.toml")
	raw, err := fsys.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if bytes.Contains(raw, []byte("[tool.upc")) {
		return fmt.Errorf("%s already has a [tool.upc] table", path)
	}
	out := bytes.Clone(raw)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(out, fmt.Sprintf("\n[tool.upc.scripts]\ndev = %q\nstart = %q\n", AppServerCommand, AppServerCommand)...)
	return fsys.WriteFileAtomic(path, out, 0o644)
}
