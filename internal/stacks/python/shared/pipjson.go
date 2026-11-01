package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func isExit(err error) bool {
	var exit *execx.ExitError
	return errors.As(err, &exit)
}

// ParsePipList reads the output of `pip list --format json` (optionally with
// --outdated), which uv's pip interface shares.
func ParsePipList(text string) ([]core.Package, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var raw []struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parse package list: %w", err)
	}
	pkgs := make([]core.Package, len(raw))
	for i, r := range raw {
		pkgs[i] = core.Package{Name: r.Name, Version: r.Version, Latest: r.LatestVersion}
	}
	return pkgs, nil
}

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
