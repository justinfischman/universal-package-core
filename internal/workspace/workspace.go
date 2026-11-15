// Package workspace holds the object model of a multi-app workspace
// (ADR-0014): the Workspace and its Apps, their start order, and the
// Orchestrator that starts and stops them. No package manager imports it.
package workspace

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/envs"
	"github.com/justinfischman/universal-package-core/internal/config"
)

// Workspace is a root directory, its declared members and its apps.
type Workspace struct {
	Root    string
	Members []string
	apps    map[string]*App
}

// Load builds the Workspace from config, validating the app tables first.
// root is the directory app paths are relative to.
func Load(root string, c config.Config) (*Workspace, error) {
	if err := config.ValidateApps(c.Apps); err != nil {
		return nil, err
	}
	w := &Workspace{Root: root, Members: c.Workspace.Members, apps: make(map[string]*App, len(c.Apps))}
	for name, a := range c.Apps {
		w.apps[name] = &App{
			Name: name, Path: a.Path, Dir: filepath.Join(root, filepath.FromSlash(a.Path)),
			Port: a.Port, URL: a.URL, Provides: a.Provides, Consumes: a.Consumes,
			Contract: a.Contract, Env: a.Env,
		}
	}
	return w, nil
}

// Names lists the app names in sorted order.
func (w *Workspace) Names() []string {
	names := make([]string, 0, len(w.apps))
	for n := range w.apps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// App returns the named app.
func (w *Workspace) App(name string) (*App, bool) {
	a, ok := w.apps[name]
	return a, ok
}

// Value implements envs.Lookup for references of the form apps.<name>.<field>.
func (w *Workspace) Value(ref string) (string, bool) {
	parts := strings.Split(ref, ".")
	if len(parts) != 3 || parts[0] != "apps" {
		return "", false
	}
	a, ok := w.apps[parts[1]]
	if !ok {
		return "", false
	}
	return a.Field(parts[2])
}

// Resolver resolves ${apps...} references against this workspace.
func (w *Workspace) Resolver() envs.EnvResolver { return envs.EnvResolver{Lookup: w} }

// ChildEnv is the env the app's process receives: its Env table with every
// reference resolved. An unresolved variable is an error.
func (w *Workspace) ChildEnv(a *App) ([]string, error) { return w.Resolver().Child(a.Env) }
