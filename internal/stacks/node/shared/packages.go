package shared

import (
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// PackageSummary renders list output: one "name version" line per package.
func PackageSummary(pkgs []core.Package) string {
	lines := make([]string, len(pkgs))
	for i, p := range pkgs {
		lines[i] = strings.TrimSpace(p.Name + " " + p.Version)
	}
	return strings.Join(lines, "\n")
}

// OutdatedPackages keeps the packages with a newer version and renders them.
func OutdatedPackages(pkgs []core.Package) ([]core.Package, string) {
	stale := []core.Package{}
	var lines []string
	for _, p := range pkgs {
		if p.Latest != "" && p.Latest != p.Version {
			stale = append(stale, p)
			lines = append(lines, fmt.Sprintf("%s %s -> %s", p.Name, p.Version, p.Latest))
		}
	}
	return stale, strings.Join(lines, "\n")
}
