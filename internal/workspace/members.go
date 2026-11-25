package workspace

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// MemberDiff is how the configured workspace members differ from the list the
// package manager reports natively (ADR-0014 decision 3).
type MemberDiff struct {
	// ConfigOnly are members the config declares and the manager does not know.
	ConfigOnly []string
	// NativeOnly are members the manager knows and the config does not declare.
	NativeOnly []string
}

// Empty reports that the two lists agree.
func (d MemberDiff) Empty() bool { return len(d.ConfigOnly) == 0 && len(d.NativeOnly) == 0 }

// String describes the difference, one clause per side.
func (d MemberDiff) String() string {
	var parts []string
	if len(d.ConfigOnly) > 0 {
		parts = append(parts, fmt.Sprintf("in config but not known to the manager: %s", strings.Join(d.ConfigOnly, ", ")))
	}
	if len(d.NativeOnly) > 0 {
		parts = append(parts, fmt.Sprintf("known to the manager but not in config: %s", strings.Join(d.NativeOnly, ", ")))
	}
	return strings.Join(parts, "; ")
}

// CompareMembers compares the configured members with the manager's native
// list as sets of cleaned, slash-separated paths; order and duplicates do not
// matter. Both results are sorted.
func CompareMembers(configured, native []string) MemberDiff {
	c, n := cleanSet(configured), cleanSet(native)
	return MemberDiff{ConfigOnly: minus(c, n), NativeOnly: minus(n, c)}
}

func cleanSet(paths []string) []string {
	var out []string
	for _, p := range paths {
		p = path.Clean(strings.ReplaceAll(p, `\`, "/"))
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func minus(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
