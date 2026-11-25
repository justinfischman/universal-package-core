package pnpm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// workspaceYAML is the part of pnpm-workspace.yaml that lists the members.
type workspaceYAML struct {
	Packages []string `yaml:"packages"`
}

// NativeMembers implements core.MemberLister from the `packages` list of
// pnpm-workspace.yaml (patterns, `!` excludes included). A missing file means
// no workspace and no members.
func (pn *Pnpm) NativeMembers(_ context.Context, dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, workspaceFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", workspaceFile, err)
	}
	var ws workspaceYAML
	if err := yaml.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("parse %s: %w", workspaceFile, err)
	}
	return ws.Packages, nil
}
