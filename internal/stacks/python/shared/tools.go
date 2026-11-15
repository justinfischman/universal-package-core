package shared

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// ErrNotConfigured means a project has not declared a tool, so the fill has
// nothing to run. Package managers report it as an Unsupported capability whose
// reason says how to enable it.
var ErrNotConfigured = errors.New("tool not configured")

// Tool is a developer tool a fill can run when the project declares it.
type Tool struct {
	Name string
	// Script is the [tool.upc.scripts] entry that overrides the default command.
	Script string
	// Command is the default command line (program first) used when the tool is
	// declared but no script overrides it.
	Command []string
	// Dependency is the distribution whose presence in the project's
	// dependencies means the tool is declared.
	Dependency string
	// ToolTable is the [tool.<name>] table whose presence declares the tool.
	ToolTable string
	// ConfigFiles are file names whose presence declares the tool.
	ConfigFiles []string
	// Enable tells the user how to turn the tool on.
	Enable string
}

// The tools behind the test and lint fills.
var (
	Pytest = Tool{
		Name: "pytest", Script: "test", Command: []string{"pytest"}, Dependency: "pytest", ToolTable: "pytest",
		ConfigFiles: []string{"pytest.ini", "conftest.py"},
		Enable:      "add pytest as a development dependency (upc add --dev pytest) or add a `test` entry to [tool.upc.scripts] in pyproject.toml",
	}
	Ruff = Tool{
		Name: "ruff", Script: "lint", Command: []string{"ruff", "check"}, Dependency: "ruff", ToolTable: "ruff",
		ConfigFiles: []string{"ruff.toml", ".ruff.toml"},
		Enable:      "add ruff as a development dependency (upc add --dev ruff) or add a `lint` entry to [tool.upc.scripts] in pyproject.toml",
	}
)

// Plan returns the command line (program first, passthrough args appended) to
// run the tool in the project. A [tool.upc.scripts] entry wins; otherwise the
// default command runs when the project declares the tool; otherwise the error
// wraps ErrNotConfigured and says how to enable it.
func (t Tool) Plan(fsys fs.FS, args []string) ([]string, error) {
	project, err := python.ReadProject(fsys)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if line, ok := project.UpcScripts[t.Script]; ok {
		words, err := scripts.Split(line)
		if err != nil {
			return nil, fmt.Errorf("%w: [tool.upc.scripts] %q: %v", core.ErrInvalidRequest, t.Script, err)
		}
		return invoke.Concat(words, args), nil
	}
	if t.declared(fsys, project) {
		return invoke.Concat(t.Command, args), nil
	}
	return nil, fmt.Errorf("%w: %s is not configured for this project; %s", ErrNotConfigured, t.Name, t.Enable)
}

func (t Tool) declared(fsys fs.FS, p python.Project) bool {
	if p.ToolTables[t.ToolTable] || p.Dependencies[t.Dependency] {
		return true
	}
	for _, f := range t.ConfigFiles {
		if _, err := fs.Stat(fsys, f); err == nil {
			return true
		}
	}
	files, _ := fs.Glob(fsys, "requirements*.txt")
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if python.RequirementName(line) == t.Dependency {
				return true
			}
		}
	}
	return false
}

// AsUnsupported turns ErrNotConfigured into the Unsupported error a package
// manager reports for capability c; other errors pass through.
func AsUnsupported(err error, pm string, c core.Capability) error {
	if errors.Is(err, ErrNotConfigured) {
		return &core.UnsupportedError{PackageManager: pm, Capability: c, Reason: strings.TrimPrefix(err.Error(), ErrNotConfigured.Error()+": ")}
	}
	return err
}
