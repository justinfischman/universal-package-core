package shared

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// DefaultScriptExt is the extension given to a script created without one.
const DefaultScriptExt = ".ts"

// scriptExts are the extensions a script file may have.
var scriptExts = map[string]bool{".ts": true, ".mts": true, ".js": true, ".mjs": true}

var (
	projectName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	invalidChar = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// ValidateProjectName accepts names safe as a directory and as an npm package
// name: lower-case letters, digits, '-' and '_', starting with a letter.
func ValidateProjectName(name string) error {
	if !projectName.MatchString(name) {
		return fmt.Errorf("%w: %q is not a valid package name (lower-case letters, digits, '-' and '_', starting with a letter)", core.ErrInvalidRequest, name)
	}
	return nil
}

// ScriptFileName turns a script name into the file to create. A bare name gets
// the .ts extension; paths, flag-like names and unknown extensions are refused.
func ScriptFileName(name string) (string, error) {
	switch {
	case strings.TrimSpace(name) == "", strings.HasPrefix(name, "-"), strings.Contains(name, ".."):
		return "", fmt.Errorf("%w: %q is not a valid script name", core.ErrInvalidRequest, name)
	case strings.ContainsAny(name, `/\`):
		return "", fmt.Errorf("%w: script name %q must not contain a path separator", core.ErrInvalidRequest, name)
	}
	ext := filepath.Ext(name)
	if ext == "" {
		return name + DefaultScriptExt, nil
	}
	if !scriptExts[ext] {
		return "", fmt.Errorf("%w: script %q must end in .ts, .mts, .js or .mjs", core.ErrInvalidRequest, name)
	}
	return name, nil
}

// PackageNameFor derives a valid npm package name from a script file name.
func PackageNameFor(file string) (string, error) {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	name := strings.Trim(invalidChar.ReplaceAllString(strings.ToLower(base), "-"), "-_")
	if name == "" {
		return "", fmt.Errorf("%w: cannot derive a package name from %q", core.ErrInvalidRequest, file)
	}
	return name, nil
}

// IsTypeScriptFile reports whether file needs type stripping to run.
func IsTypeScriptFile(file string) bool {
	switch filepath.Ext(file) {
	case ".ts", ".mts", ".cts":
		return true
	}
	return false
}
