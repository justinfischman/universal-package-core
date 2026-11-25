package uv

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// uvWorkspace is `[tool.uv.workspace]` of pyproject.toml.
type uvWorkspace struct {
	Members []string `toml:"members"`
	Exclude []string `toml:"exclude"`
}

type uvPyproject struct {
	Tool struct {
		UV struct {
			Workspace uvWorkspace `toml:"workspace"`
		} `toml:"uv"`
	} `toml:"tool"`
}

// NativeMembers implements core.MemberLister from `[tool.uv.workspace]` in
// pyproject.toml: its `members` globs, then each `exclude` glob as a `!` pattern.
// A project without the table has no members.
func (u *UV) NativeMembers(_ context.Context, dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no pyproject.toml in %s to read the uv workspace from", dir)
	}
	if err != nil {
		return nil, fmt.Errorf("read pyproject.toml: %w", err)
	}
	var p uvPyproject
	if err := toml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse pyproject.toml: %w", err)
	}
	ws := p.Tool.UV.Workspace
	out := append([]string{}, ws.Members...)
	for _, e := range ws.Exclude {
		out = append(out, "!"+e)
	}
	return out, nil
}
