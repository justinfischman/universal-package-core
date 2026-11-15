package shared

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/drift"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
)

// Installed lists the distributions installed in the project's virtual
// environment as normalized name -> version, read from the *.dist-info
// directory names in its site-packages. ok is false when the environment
// has no site-packages to read (no venv, or an unknown layout).
func Installed(fsys fs.FS) (installed map[string]string, ok bool) {
	dirs, _ := fs.Glob(fsys, VenvDir+"/lib/python*/site-packages")
	dirs = append(dirs, VenvDir+"/Lib/site-packages")
	installed = map[string]string{}
	for _, dir := range dirs {
		entries, err := fs.ReadDir(fsys, dir)
		if err != nil {
			continue
		}
		ok = true
		for _, e := range entries {
			stem, found := strings.CutSuffix(e.Name(), ".dist-info")
			if !found {
				continue
			}
			if i := strings.LastIndex(stem, "-"); i > 0 {
				installed[python.NormalizeName(stem[:i])] = stem[i+1:]
			}
		}
	}
	return installed, ok
}

// EnvDrift compares what a project file wants (normalized name -> exact
// version, or "" when any version satisfies it) with what the virtual
// environment holds. source names the file in messages. It reports nothing
// when the environment cannot be read: a missing venv is a separate finding.
func EnvDrift(fsys fs.FS, source string, want map[string]string) []core.Finding {
	installed, ok := Installed(fsys)
	if !ok {
		return nil
	}
	var missing, stale []string
	for name, version := range want {
		have, present := installed[name]
		switch {
		case !present:
			missing = append(missing, name)
		case version != "" && have != version:
			stale = append(stale, fmt.Sprintf("%s (%s installed, %s in %s)", name, have, version, source))
		}
	}
	return drift.Findings(VenvDir, source, missing, stale)
}

// RequirementPins reads a requirements file and the files it includes with
// -r/--requirement into normalized name -> exact version ("" when not
// pinned with ==). Lines that depend on the machine (environment markers),
// install from a path or URL, or are options are skipped: they cannot be
// checked against the environment by name alone.
func RequirementPins(fsys fs.FS, name string) map[string]string {
	pins := map[string]string{}
	collectPins(fsys, name, pins, nil)
	return pins
}

func collectPins(fsys fs.FS, name string, pins map[string]string, seen []string) {
	name = path.Clean(name)
	if slices.Contains(seen, name) || !fs.ValidPath(name) {
		return
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		for _, flag := range []string{"-r", "--requirement"} {
			if rest, ok := strings.CutPrefix(line, flag); ok && rest != "" && strings.ContainsAny(rest[:1], " =") {
				collectPins(fsys, path.Join(path.Dir(name), strings.TrimLeft(rest, " =")), pins, append(seen, name))
			}
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") ||
			strings.ContainsAny(line, ";@/\\") || strings.Contains(line, "://") {
			continue
		}
		req := python.RequirementName(line)
		if req == "" {
			continue
		}
		rest := ""
		if i := strings.IndexAny(line, " <>=!~[("); i >= 0 {
			rest = line[i:]
		}
		pins[req] = exactVersion(rest)
	}
}

// exactVersion returns X for a specifier that pins "==X", else "".
func exactVersion(rest string) string {
	_, spec, found := strings.Cut(rest, "==")
	if !found || strings.HasPrefix(spec, "=") {
		return ""
	}
	spec = strings.TrimSpace(strings.SplitN(spec, ",", 2)[0])
	if spec == "" || strings.ContainsAny(spec, "* ") {
		return ""
	}
	return spec
}
