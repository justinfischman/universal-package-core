package config

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// App is one `[apps.<name>]` table (ADR-0014): a member project that upc can
// start, with what it offers and what it needs.
type App struct {
	// Path is the app directory relative to the workspace root.
	Path string `toml:"path,omitempty"`
	Port int    `toml:"port,omitempty"`
	// URL may hold ${apps.<name>.<field>} references.
	URL string `toml:"url,omitempty"`
	// Provides lists what the app offers its consumers (for example "http").
	Provides []string `toml:"provides,omitempty"`
	// Consumes lists the names of the apps this one needs.
	Consumes []string `toml:"consumes,omitempty"`
	// Contract is the path of the contract file, relative to the app.
	Contract string `toml:"contract,omitempty"`
	// Env is extra child environment; values may hold ${apps...} references.
	Env map[string]string `toml:"env,omitempty"`
}

var appNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidateApps checks the app tables without touching the disk: names are
// plain words and unique ignoring case, paths stay inside the workspace and are
// unique, ports are in range, and every consumer target is a declared app.
func ValidateApps(apps map[string]App) error {
	names := make([]string, 0, len(apps))
	for n := range apps {
		names = append(names, n)
	}
	sort.Strings(names)
	lower, paths := map[string]string{}, map[string]string{}
	for _, name := range names {
		if !appNamePattern.MatchString(name) {
			return fmt.Errorf("%w: invalid app name %q (use letters, digits, '-' and '_')", core.ErrInvalidRequest, name)
		}
		if other, dup := lower[strings.ToLower(name)]; dup {
			return fmt.Errorf("%w: duplicate app name %q (same as %q ignoring case)", core.ErrInvalidRequest, name, other)
		}
		lower[strings.ToLower(name)] = name
	}
	for _, name := range names {
		a := apps[name]
		if err := validateAppPath(name, a.Path); err != nil {
			return err
		}
		if other, dup := paths[path.Clean(a.Path)]; dup {
			return fmt.Errorf("%w: apps %q and %q share path %q", core.ErrInvalidRequest, other, name, a.Path)
		}
		paths[path.Clean(a.Path)] = name
		if a.Port < 0 || a.Port > 65535 {
			return fmt.Errorf("%w: app %q port %d is out of range", core.ErrInvalidRequest, name, a.Port)
		}
		for _, target := range a.Consumes {
			if _, ok := apps[target]; !ok {
				return fmt.Errorf("%w: app %q consumes unknown app %q", core.ErrInvalidRequest, name, target)
			}
			if target == name {
				return fmt.Errorf("%w: app %q cannot consume itself", core.ErrInvalidRequest, name)
			}
		}
	}
	return nil
}

func validateAppPath(name, p string) error {
	clean := path.Clean(p)
	if p == "" || path.IsAbs(p) || strings.Contains(p, `\`) || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%w: app %q path %q must be a relative path inside the workspace", core.ErrInvalidRequest, name, p)
	}
	return nil
}

// mergeApps returns base with override's apps added; an app in both is replaced.
func mergeApps(base, override map[string]App) map[string]App {
	if len(override) == 0 {
		return base
	}
	out := make(map[string]App, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
