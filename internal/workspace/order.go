package workspace

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrCycle means the apps' consumes relations form a loop.
var ErrCycle = errors.New("app dependency cycle")

// Order lists every app so each one comes after the apps it consumes. Ties
// break by name, so the order is deterministic. A cycle is an error naming it.
func (w *Workspace) Order() ([]*App, error) {
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var path []string
	var out []*App
	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case done:
			return nil
		case visiting:
			return cycleError(path, name)
		}
		state[name] = visiting
		path = append(path, name)
		a := w.apps[name]
		deps := append([]string(nil), a.Consumes...)
		sort.Strings(deps)
		for _, d := range deps {
			if err := visit(d); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[name] = done
		out = append(out, a)
		return nil
	}
	for _, name := range w.Names() {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// cycleError names the loop that closes when name is reached again.
func cycleError(path []string, name string) error {
	for i, n := range path {
		if n == name {
			loop := append(append([]string{}, path[i:]...), name)
			return fmt.Errorf("%w: %s", ErrCycle, strings.Join(loop, " -> "))
		}
	}
	return fmt.Errorf("%w: %s", ErrCycle, name)
}
