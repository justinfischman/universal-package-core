package shared

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// VenvDir is the project-local virtual environment directory.
const VenvDir = ".venv"

// SystemPython is the interpreter used to create a virtual environment.
func SystemPython(goos string) string {
	if goos == "windows" {
		return "python"
	}
	return "python3"
}

func venvBin(goos string) (dir, exe string) {
	if goos == "windows" {
		return VenvDir + "/Scripts", ".exe"
	}
	return VenvDir + "/bin", ""
}

// VenvProgram returns the project-relative path of a program inside the
// virtual environment, and whether it exists. The path is relative to the
// project directory, which is the command's working directory.
func VenvProgram(fsys fs.FS, goos, name string) (string, bool) {
	dir, exe := venvBin(goos)
	p := path.Join(dir, name+exe)
	info, err := fs.Stat(fsys, p)
	return p, err == nil && !info.IsDir()
}

// HasVenv reports whether the project has a virtual environment.
func HasVenv(fsys fs.FS) bool {
	info, err := fs.Stat(fsys, VenvDir)
	return err == nil && info.IsDir()
}

// Python returns the interpreter to run project code with: the virtual
// environment's when present, else the system interpreter.
func Python(fsys fs.FS, goos string) string {
	if p, ok := VenvProgram(fsys, goos, "python"); ok {
		return p
	}
	return SystemPython(goos)
}

// Program resolves a program name for a project: the virtual environment's copy
// when it exists, else the bare name looked up on PATH.
func Program(fsys fs.FS, goos, name string) string {
	if p, ok := VenvProgram(fsys, goos, name); ok {
		return p
	}
	return name
}

// RequireModule checks that a Python module is importable by python and, when it
// is not, returns a ToolMissingError whose install command adds package pkg, so
// the caller can offer the install and re-run.
func (i *Invocation) RequireModule(python, module, pkg string) error {
	code := fmt.Sprintf("import importlib.util,sys; sys.exit(0 if importlib.util.find_spec(%q) else 1)", module)
	if _, err := i.RunProgram(false, python, "-c", code); err != nil {
		if isExit(err) {
			cmd := []string{python, "-m", "pip", "install", pkg}
			return &core.ToolMissingError{Tool: pkg, InstallHint: strings.Join(cmd, " "), InstallCmd: cmd}
		}
		return err
	}
	return nil
}

// VenvPythonPath is where the environment's python lives, whether or not the
// environment exists yet.
func VenvPythonPath(goos string) string {
	dir, exe := venvBin(goos)
	return path.Join(dir, "python"+exe)
}
