package pip

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// A requirement line is `name[extras]<rest>` where rest starts with a version
// operator, an environment marker, a direct reference or a comment. Lines that
// do not match (options such as -r, URLs, paths, comments) are kept untouched.
var (
	specRe = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)(\[[^\]]*\])?\s*(.*)$`)
	// A simple pin: ==version followed by nothing, a marker or a comment.
	pinRe     = regexp.MustCompile(`^(\s*[A-Za-z0-9][A-Za-z0-9._-]*(?:\[[^\]]*\])?\s*==\s*)([^\s;,#]+)(\s*(?:[;#].*)?)$`)
	groupRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	restStart = regexp.MustCompile(`^(?:$|[=<>!~;@#])`)
)

// spec is a parsed requirement specifier.
type spec struct {
	name   string // spelling as written
	key    string // normalized name
	extras string // "[a,b]" or ""
	marker string // text after ";" without the semicolon, or ""
}

// parseSpec splits a specifier into name, extras and marker. ok is false for
// anything that is not a plain named requirement (URLs, paths, options).
func parseSpec(raw string) (spec, bool) {
	m := specRe.FindStringSubmatch(raw)
	if m == nil || !restStart.MatchString(m[3]) {
		return spec{}, false
	}
	s := spec{name: m[1], key: python.NormalizeName(m[1]), extras: m[2]}
	if _, marker, ok := strings.Cut(m[3], ";"); ok {
		marker, _, _ = strings.Cut(marker, "#")
		s.marker = strings.TrimSpace(marker)
	}
	return s, true
}

// pinned renders the spec pinned to version, keeping extras and marker.
func (s spec) pinned(version string) string {
	line := s.name + s.extras + "==" + version
	if s.marker != "" {
		line += "; " + s.marker
	}
	return line
}

// requirementsFile is the group's file: requirements.txt, requirements-dev.txt,
// requirements-<group>.txt.
func requirementsFile(group string) (string, error) {
	switch {
	case group == "":
		return "requirements.txt", nil
	case !groupRe.MatchString(group):
		return "", fmt.Errorf("%w: %q is not a valid group", core.ErrInvalidRequest, group)
	}
	return "requirements-" + group + ".txt", nil
}

// requirements is an editable requirements file. It changes only the lines it
// is asked to; comments, options and ordering are preserved.
type requirements struct {
	lines []string
}

func parseRequirements(data []byte) *requirements {
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return &requirements{}
	}
	return &requirements{lines: strings.Split(text, "\n")}
}

// bytes renders the file with a single trailing newline (empty stays empty).
func (r *requirements) bytes() []byte {
	if len(r.lines) == 0 {
		return nil
	}
	return []byte(strings.Join(r.lines, "\n") + "\n")
}

func (r *requirements) indexOf(key string) int {
	for i, l := range r.lines {
		if s, ok := parseSpec(l); ok && s.key == key {
			return i
		}
	}
	return -1
}

// set replaces the requirement named key, or appends line if there is none.
func (r *requirements) set(key, line string) {
	if i := r.indexOf(key); i >= 0 {
		r.lines[i] = line
		return
	}
	for _, l := range r.lines {
		if l == line {
			return
		}
	}
	r.lines = append(r.lines, line)
}

// remove deletes every requirement whose normalized name is in keys.
func (r *requirements) remove(keys map[string]bool) {
	kept := r.lines[:0:0]
	for _, l := range r.lines {
		if s, ok := parseSpec(l); ok && keys[s.key] {
			continue
		}
		kept = append(kept, l)
	}
	r.lines = kept
}

// repin moves simple `name==version` pins to the installed versions. only, when
// non-empty, limits the change to those normalized names. Ranges, markers-only
// lines and anything that is not a plain pin are left alone.
func (r *requirements) repin(installed map[string]string, only map[string]bool) {
	for i, l := range r.lines {
		s, ok := parseSpec(l)
		if !ok || (len(only) > 0 && !only[s.key]) {
			continue
		}
		version, known := installed[s.key]
		if !known {
			continue
		}
		if m := pinRe.FindStringSubmatch(l); m != nil {
			r.lines[i] = m[1] + version + m[3]
		}
	}
}

// parseFreeze reads `pip freeze` into normalized name -> version for `==` pins.
// Direct references (`name @ url`) and editable installs have no version.
func parseFreeze(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		name, version, ok := strings.Cut(strings.TrimSpace(line), "==")
		if !ok || strings.HasPrefix(name, "-") {
			continue
		}
		out[python.NormalizeName(name)] = version
	}
	return out
}
