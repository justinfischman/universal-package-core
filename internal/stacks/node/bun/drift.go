package bun

import (
	"io/fs"
	"regexp"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// lockDrift compares bun.lock with node_modules. An unreadable lock, and the
// legacy binary bun.lockb, report nothing.
func lockDrift(fsys fs.FS) []core.Finding {
	data, err := fs.ReadFile(fsys, lockFile)
	if err != nil {
		return nil
	}
	return shared.EnvDrift(fsys, lockFile, lockedVersions(string(data)))
}

// lockEntry matches a top-level `"name": ["name@version", ...]` line of
// bun.lock (a JSONC file). Nested copies are keyed `"parent/name"` and skipped:
// only the version hoisted to the top of node_modules is compared.
var lockEntry = regexp.MustCompile(`(?m)^\s+"((?:@[^/"]+/)?[^/"]+)":\s*\["(?:@[^/"]+/)?[^@"]+@([^"]+)"`)

// lockedVersions reads package name -> locked versions from bun.lock.
func lockedVersions(text string) map[string][]string {
	out := map[string][]string{}
	for _, m := range lockEntry.FindAllStringSubmatch(text, -1) {
		out[m[1]] = append(out[m[1]], m[2])
	}
	return out
}
