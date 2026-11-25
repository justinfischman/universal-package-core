package workspace

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

// skippedDirs are never searched when a pattern is expanded.
var skippedDirs = []string{"node_modules", ".git", ".venv", "__pycache__"}

// ExpandMembers turns member patterns into the directories they name under
// root, as sorted slash-separated paths. A pattern holds `*`, `?`, `[...]` or
// `**` (any number of directories); one without those names its directory as
// written, whether or not it exists. A pattern starting with `!` removes what
// the other patterns matched, as the managers' own exclude lists do.
func ExpandMembers(root fs.FS, patterns []string) ([]string, error) {
	var dirs []string
	var excludes []string
	for _, p := range patterns {
		p = cleanPattern(p)
		if rest, ok := strings.CutPrefix(p, "!"); ok {
			excludes = append(excludes, cleanPattern(rest))
		}
	}
	var all []string
	walked := false
	for _, p := range patterns {
		p = cleanPattern(p)
		if p == "" || strings.HasPrefix(p, "!") {
			continue
		}
		if !strings.ContainsAny(p, "*?[") {
			dirs = append(dirs, p)
			continue
		}
		if !walked {
			var err error
			if all, err = listDirs(root); err != nil {
				return nil, err
			}
			walked = true
		}
		for _, d := range all {
			if matchSegments(strings.Split(p, "/"), strings.Split(d, "/")) {
				dirs = append(dirs, d)
			}
		}
	}
	var out []string
	for _, d := range dirs {
		if excluded(excludes, d) || slices.Contains(out, d) {
			continue
		}
		out = append(out, d)
	}
	slices.Sort(out)
	return out, nil
}

func cleanPattern(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	neg := strings.HasPrefix(p, "!")
	p = path.Clean(strings.TrimPrefix(p, "!"))
	if p == "." {
		p = ""
	}
	if neg {
		return "!" + p
	}
	return p
}

func excluded(excludes []string, dir string) bool {
	for _, e := range excludes {
		if matchSegments(strings.Split(e, "/"), strings.Split(dir, "/")) {
			return true
		}
	}
	return false
}

// listDirs returns every directory under root except the skipped ones.
func listDirs(root fs.FS) ([]string, error) {
	var dirs []string
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || p == "." {
			return nil
		}
		if slices.Contains(skippedDirs, d.Name()) {
			return fs.SkipDir
		}
		dirs = append(dirs, p)
		return nil
	})
	return dirs, err
}

// matchSegments matches path segments against pattern segments; `**` matches
// zero or more whole segments.
func matchSegments(pat, segs []string) bool {
	if len(pat) == 0 {
		return len(segs) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if matchSegments(pat[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], segs[0])
	return err == nil && ok && matchSegments(pat[1:], segs[1:])
}
