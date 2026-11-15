package core

import (
	"fmt"
	"strings"
)

// Placeholders a CommandSpec template may use. The list is closed on purpose:
// a spec is an argv template, not a scripting language.
const (
	PlaceholderPkg        = "{pkg}"        // one argv element per package
	PlaceholderConstraint = "{constraint}" // the rendered constraint, only inside a {pkg} element
	PlaceholderGroup      = "{group}"      // the whole element expands to the resolved group argv
	PlaceholderScript     = "{script}"     // the whole element expands to the script-mode argv
	PlaceholderDir        = "{dir}"        // the project directory, inside any element
)

// GroupSpec maps upc's dependency group to a manager's flags. Dev, when set,
// is used for the group "dev"; Named is used for every other group (and for
// "dev" when Dev is nil) with "{group}" replaced by the group name.
type GroupSpec struct {
	Dev   []string
	Named []string
}

// Args resolves a group name to argv. The empty group yields no argv. A name
// that starts with "-" is rejected so it cannot be read as a flag.
func (g GroupSpec) Args(group string) ([]string, error) {
	if group == "" {
		return nil, nil
	}
	if strings.HasPrefix(group, "-") {
		return nil, fmt.Errorf("%w: %q is not a valid group", ErrInvalidRequest, group)
	}
	if group == "dev" && g.Dev != nil {
		return append([]string(nil), g.Dev...), nil
	}
	out := make([]string, len(g.Named))
	for i, a := range g.Named {
		out[i] = strings.ReplaceAll(a, PlaceholderGroup, group)
	}
	return out, nil
}

// CommandSpec is an argv template for one manager command. The program is not
// part of it (the manager's binary is). Elements are never joined into a shell
// string, so values need no quoting.
type CommandSpec struct {
	Argv []string
	// PerPackage is emitted before every package expanded from a {pkg}
	// element, for tools that want a flag per package (--upgrade-package p).
	PerPackage []string
	// Group resolves {group}; it is only consulted when Argv has a {group}.
	Group GroupSpec
}

// Vars are the values a template is expanded with.
type Vars struct {
	Packages   []string
	Constraint string   // rendered by Constraint.Render; "" when none was requested
	GroupName  string   // upc's group name; resolved through CommandSpec.Group
	Script     []string // argv for script mode, spliced at {script}
	Dir        string
}

// Expand fills the template. Every {pkg} element repeats once per package, with
// {constraint} replaced inside it. A constraint that the template has nowhere
// to place is an error rather than silently dropped.
func (s CommandSpec) Expand(v Vars) ([]string, error) {
	if v.Constraint != "" && !s.accepts(PlaceholderConstraint) {
		return nil, fmt.Errorf("%w: this command does not take a version constraint", ErrInvalidRequest)
	}
	var out []string
	for _, el := range s.Argv {
		switch {
		case el == PlaceholderGroup:
			g, err := s.Group.Args(v.GroupName)
			if err != nil {
				return nil, err
			}
			out = append(out, g...)
		case el == PlaceholderScript:
			out = append(out, v.Script...)
		case strings.Contains(el, PlaceholderPkg):
			for _, p := range v.Packages {
				out = append(out, s.PerPackage...)
				out = append(out, fill(el, p, v))
			}
		default:
			out = append(out, fill(el, "", v))
		}
	}
	return out, nil
}

func (s CommandSpec) accepts(placeholder string) bool {
	for _, el := range s.Argv {
		if strings.Contains(el, PlaceholderPkg) && strings.Contains(el, placeholder) {
			return true
		}
	}
	return false
}

func fill(el, pkg string, v Vars) string {
	r := strings.NewReplacer(PlaceholderPkg, pkg, PlaceholderConstraint, v.Constraint, PlaceholderDir, v.Dir)
	return r.Replace(el)
}
