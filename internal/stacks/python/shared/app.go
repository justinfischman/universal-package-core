package shared

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
)

// AppCommands maps an app kind to the command its dev and start scripts run;
// the files are in templates/app, templates/app-cli and templates/app-library
// of the python language. The empty kind is the default service.
var AppCommands = map[string]string{
	"":        "python server.py",
	"cli":     "python cli.py",
	"library": "python demo.py",
}

// AppendAppScripts adds the dev and start commands to the [tool.upc.scripts]
// table of the pyproject.toml in projectDir, so `upc up` can start the app.
// It is for a freshly scaffolded project: an existing [tool.upc] table is
// refused rather than merged, and so is a kind with no command. All writes go
// through fsys.
func AppendAppScripts(fsys fsx.FS, projectDir, kind string) error {
	command, ok := AppCommands[kind]
	if !ok {
		return fmt.Errorf("unknown app kind %q", kind)
	}
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
	out = append(out, fmt.Sprintf("\n[tool.upc.scripts]\ndev = %q\nstart = %q\n", command, command)...)
	return fsys.WriteFileAtomic(path, out, 0o644)
}
