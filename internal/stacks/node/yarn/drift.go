package yarn

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// lockDrift compares yarn.lock with node_modules. An unreadable lock reports
// nothing.
func lockDrift(fsys fs.FS) []core.Finding {
	data, err := fs.ReadFile(fsys, lockFile)
	if err != nil {
		return nil
	}
	return shared.EnvDrift(fsys, lockFile, lockedVersions(string(data)))
}

// lockedVersions reads package name -> locked versions from either yarn.lock
// format. Classic entries read `name@range, name@range:` then `  version "x"`;
// berry entries quote the header and write `  version: x`.
func lockedVersions(text string) map[string][]string {
	out := map[string][]string{}
	var names []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " "):
			names = entryNames(strings.TrimSuffix(line, ":"))
		case strings.HasPrefix(line, "  version"):
			version := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(line, "  version"), ":"), ` "`)
			for _, n := range names {
				out[n] = append(out[n], version)
			}
			names = nil
		}
	}
	return out
}

// entryNames returns the distinct package names in an entry header.
func entryNames(header string) []string {
	var names []string
	for _, spec := range strings.Split(header, ",") {
		spec = strings.Trim(strings.TrimSpace(spec), `"`)
		if spec == "" {
			continue
		}
		i := strings.Index(spec[1:], "@") // skips a scope's own "@"
		if i < 0 {
			continue
		}
		if name := spec[:i+1]; !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}
