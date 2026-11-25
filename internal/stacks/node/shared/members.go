package shared

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// NativeMembers returns the workspace patterns npm, Yarn and bun read from the
// `workspaces` field of dir's package.json. A manifest without the field has no
// members; a missing manifest is an error.
func NativeMembers(dir string) ([]string, error) {
	members, err := WorkspacesOf(os.DirFS(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no package.json in %s to read workspaces from", dir)
		}
		return nil, fmt.Errorf("read workspaces: %w", err)
	}
	return members, nil
}
