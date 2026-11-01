package shared

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ScriptExt is the file extension of a Python script.
const ScriptExt = ".py"

// ValidateName rejects names that are empty, flag-like or escape the project.
func ValidateName(name string) error {
	switch {
	case strings.TrimSpace(name) == "", strings.HasPrefix(name, "-"):
		return fmt.Errorf("%w: %q is not a valid name", core.ErrInvalidRequest, name)
	case filepath.IsAbs(name), strings.Contains(name, ".."):
		return fmt.Errorf("%w: %q must be a path inside the project", core.ErrInvalidRequest, name)
	}
	return nil
}

// ScriptFileName turns a script name into the file uv creates. A bare name
// gets the .py extension; paths and flag-like names are refused.
func ScriptFileName(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("%w: script name %q must not contain a path separator", core.ErrInvalidRequest, name)
	}
	if filepath.Ext(name) == "" {
		name += ScriptExt
	}
	return name, nil
}

var (
	projectName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	nonIdent    = regexp.MustCompile(`[^a-z0-9_]+`)
)

// ValidateProjectName accepts names safe as a directory and as a distribution
// name: letters, digits, '-' and '_', starting with a letter.
func ValidateProjectName(name string) error {
	if !projectName.MatchString(name) {
		return fmt.Errorf("%w: %q is not a valid package name (letters, digits, '-' and '_', starting with a letter)", core.ErrInvalidRequest, name)
	}
	return nil
}

// ModuleName is the importable name of a project: lower case, '-' as '_'.
func ModuleName(name string) string {
	return strings.Trim(nonIdent.ReplaceAllString(strings.ToLower(name), "_"), "_")
}
