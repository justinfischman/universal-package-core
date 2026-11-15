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
