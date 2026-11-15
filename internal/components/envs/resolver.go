package envs

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Errors returned by EnvResolver.
var (
	// ErrUnresolved means a ${...} variable has no value. It is never replaced
	// with an empty string.
	ErrUnresolved = errors.New("unresolved variable")
	// ErrCycle means variables refer to each other in a loop.
	ErrCycle = errors.New("variable cycle")
)

var variablePattern = regexp.MustCompile(`\$\{([^}]*)\}`)

// Lookup supplies the raw (not yet resolved) value of a reference such as
// "apps.api.url". It reports false when the reference names nothing or the
// value is empty. The workspace implements it, so this package does not depend
// on the workspace.
type Lookup interface {
	Value(ref string) (string, bool)
}

// EnvResolver resolves ${apps.<name>.<field>} references in config values and
// computes the env a child process receives (ADR-0014 decision 4).
type EnvResolver struct {
	Lookup Lookup
}

// Resolve replaces every ${ref} in text. A value that itself holds references
// is resolved in turn. An unknown reference is ErrUnresolved naming it; a loop
// is ErrCycle naming the chain.
func (r EnvResolver) Resolve(text string) (string, error) {
	return r.resolve(text, nil)
}

func (r EnvResolver) resolve(text string, chain []string) (string, error) {
	var firstErr error
	out := variablePattern.ReplaceAllStringFunc(text, func(match string) string {
		if firstErr != nil {
			return match
		}
		ref := strings.TrimSpace(match[2 : len(match)-1])
		v, err := r.value(ref, chain)
		if err != nil {
			firstErr = err
			return match
		}
		return v
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func (r EnvResolver) value(ref string, chain []string) (string, error) {
	for i, seen := range chain {
		if seen == ref {
			loop := append(append([]string{}, chain[i:]...), ref)
			return "", fmt.Errorf("%w: ${%s}", ErrCycle, strings.Join(loop, "} -> ${"))
		}
	}
	if r.Lookup == nil {
		return "", fmt.Errorf("%w: ${%s}", ErrUnresolved, ref)
	}
	raw, ok := r.Lookup.Value(ref)
	if !ok {
		return "", fmt.Errorf("%w: ${%s}", ErrUnresolved, ref)
	}
	return r.resolve(raw, append(append([]string{}, chain...), ref))
}

// Child resolves every value of vars and returns KEY=VALUE entries sorted by
// key, ready to append to a child process environment. It fails on the first
// key (in sorted order) whose value cannot be resolved.
func (r EnvResolver) Child(vars map[string]string) ([]string, error) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v, err := r.Resolve(vars[k])
		if err != nil {
			return nil, fmt.Errorf("env %s: %w", k, err)
		}
		out = append(out, k+"="+v)
	}
	return out, nil
}
