package shared

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Dependency groups for npm and yarn (ADR-0015, option B): a group that is not
// one of the manager's own flags (dev, optional, peer) is a workspace member
// `.universal-package-core/groups/<name>/package.json`.
const (
	// GroupsDir is the directory that holds one member per group.
	GroupsDir = ".universal-package-core/groups"
	// GroupsGlob is the workspace pattern that registers every group member. It
	// is a literal prefix, never `**`.
	GroupsGlob = GroupsDir + "/*"
	// MemberPrefix starts every group member's package name, so names stay unique.
	MemberPrefix = "upc-group-"
)

var groupName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// IsBuiltinGroup reports whether the manager itself knows the group (a flag of
// add/remove, stored in the root manifest).
func IsBuiltinGroup(group string) bool {
	return group == "dev" || group == "optional" || group == "peer"
}

// ValidateGroupName accepts names safe as a directory and as an npm package
// name suffix: lower-case letters, digits, '-' and '_', starting with a letter
// or digit.
func ValidateGroupName(group string) error {
	if !groupName.MatchString(group) {
		return fmt.Errorf("%w: %q is not a valid dependency group name (lower-case letters, digits, '-' and '_')", core.ErrInvalidRequest, group)
	}
	return nil
}

// CustomGroups returns the groups that are workspace members, in order. It
// rejects any name that is neither built in nor valid.
func CustomGroups(groups []string) ([]string, error) {
	var out []string
	for _, g := range groups {
		if IsBuiltinGroup(g) {
			continue
		}
		if err := ValidateGroupName(g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

// GroupDir is the member's directory relative to the project root, with '/'.
func GroupDir(group string) string { return path.Join(GroupsDir, group) }

// MemberName is the package name of the group's member.
func MemberName(group string) string { return MemberPrefix + group }

// MemberNames returns MemberName for each group.
func MemberNames(groups []string) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = MemberName(g)
	}
	return out
}

// GroupDirs returns GroupDir for each group.
func GroupDirs(groups []string) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = GroupDir(g)
	}
	return out
}

// HasMember reports whether the group's member manifest exists under root.
func HasMember(root fs.FS, group string) bool {
	_, err := fs.Stat(root, path.Join(GroupDir(group), "package.json"))
	return err == nil
}

// RequireMembers fails when a group has no member yet: installing a group
// that was never added would silently install nothing.
func RequireMembers(root fs.FS, groups []string) error {
	for _, g := range groups {
		if !HasMember(root, g) {
			return fmt.Errorf("%w: dependency group %q has no member %s; add a package with `--group %s` first", core.ErrInvalidRequest, g, GroupDir(g), g)
		}
	}
	return nil
}

// EnsureMember creates the group's member manifest (when missing) and
// registers GroupsGlob in the root manifest's workspaces. dir is the project
// root on the real filesystem.
func EnsureMember(fsys fsx.FS, dir, group string) error {
	if err := ValidateGroupName(group); err != nil {
		return err
	}
	member := filepath.Join(dir, filepath.FromSlash(GroupDir(group)), "package.json")
	if _, err := fsys.Stat(member); err != nil {
		data, mErr := json.MarshalIndent(map[string]any{"name": MemberName(group), "version": "0.0.0", "private": true}, "", "  ")
		if mErr != nil {
			return mErr
		}
		if err := fsys.MkdirAll(filepath.Dir(member), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(member), err)
		}
		if err := fsys.WriteFileAtomic(member, append(data, '\n'), 0o644); err != nil {
			return fmt.Errorf("create %s: %w", member, err)
		}
	}
	return RegisterWorkspace(fsys, filepath.Join(dir, "package.json"), GroupsGlob)
}
