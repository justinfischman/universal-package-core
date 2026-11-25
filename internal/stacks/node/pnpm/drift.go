package pnpm

import (
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/shared"
)

// lockDrift compares pnpm-lock.yaml with node_modules. An unreadable lock
// reports nothing.
func lockDrift(fsys fs.FS) []core.Finding {
	data, err := fs.ReadFile(fsys, lockFile)
	if err != nil {
		return nil
	}
	return shared.EnvDrift(fsys, lockFile, lockedVersions(string(data)))
}

// lockedVersions reads package name -> locked versions from the `packages:`
// section of pnpm-lock.yaml. Entries read `  name@1.2.3:` (lock format 9) or
// `  /name@1.2.3:` (earlier formats), optionally quoted and followed by a
// `(peer@1)` suffix.
func lockedVersions(text string) map[string][]string {
	out := map[string][]string{}
	inPackages := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " "):
			inPackages = line == "packages:"
		case inPackages && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   "):
			if name, version, ok := entry(strings.TrimSuffix(strings.TrimSpace(line), ":")); ok {
				out[name] = append(out[name], version)
			}
		}
	}
	return out
}

// entry splits a `name@version` key, tolerating quotes, a leading slash and a
// peer-dependency suffix.
func entry(key string) (name, version string, ok bool) {
	key = strings.TrimPrefix(strings.Trim(key, `'"`), "/")
	if i := strings.Index(key, "("); i >= 0 {
		key = key[:i]
	}
	if len(key) < 2 {
		return "", "", false
	}
	i := strings.LastIndex(key[1:], "@") // skips a scope's own "@"
	if i < 0 {
		return "", "", false
	}
	return key[:i+1], key[i+2:], true
}
