package core

import (
	"fmt"
	"sort"
)

// Registry holds modules of one kind, keyed by id. It is filled only by the
// composition root (internal/register).
type Registry[T Module] struct {
	kind  Kind
	items map[string]T
}

// NewRegistry creates an empty registry for a kind.
func NewRegistry[T Module](kind Kind) *Registry[T] {
	return &Registry[T]{kind: kind, items: make(map[string]T)}
}

// Register adds a module, rejecting empty ids, kind mismatches and duplicates.
func (r *Registry[T]) Register(m T) error {
	id := m.ID()
	if id == "" {
		return fmt.Errorf("%w: %s module has empty id", ErrInvalidRequest, r.kind)
	}
	if m.Kind() != r.kind {
		return fmt.Errorf("%w: %q is kind %q, registry holds %q", ErrInvalidRequest, id, m.Kind(), r.kind)
	}
	if _, dup := r.items[id]; dup {
		return fmt.Errorf("%w: duplicate %s id %q", ErrInvalidRequest, r.kind, id)
	}
	r.items[id] = m
	return nil
}

// Get returns the module with the id.
func (r *Registry[T]) Get(id string) (T, bool) {
	m, ok := r.items[id]
	return m, ok
}

// List returns all modules sorted by id.
func (r *Registry[T]) List() []T {
	ids := r.IDs()
	out := make([]T, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.items[id])
	}
	return out
}

// IDs returns all ids sorted; a nil registry holds none.
func (r *Registry[T]) IDs() []string {
	if r == nil {
		return nil
	}
	ids := make([]string, 0, len(r.items))
	for id := range r.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Registries bundles one registry per kind.
type Registries struct {
	Languages       *Registry[Language]
	PackageManagers *Registry[PackageManager]
	ProjectTypes    *Registry[ProjectType]
	// DefaultLanguage is the language id used when none is chosen and prompts
	// are disabled. Set only by the composition root.
	DefaultLanguage string
	// RunSources are consulted in order after the manifest sources (ADR-0016).
	RunSources []RunSource
	// UpProviders are consulted in order when no script starts the project.
	UpProviders []UpProvider
}

// NewRegistries creates empty registries.
func NewRegistries() Registries {
	return Registries{
		Languages:       NewRegistry[Language](KindLanguage),
		PackageManagers: NewRegistry[PackageManager](KindPackageManager),
		ProjectTypes:    NewRegistry[ProjectType](KindProjectType),
	}
}

// ModuleIDs returns every registered id across kinds, sorted and de-duplicated.
// The architecture test uses it as its forbidden-word list.
func (r Registries) ModuleIDs() []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range [][]string{r.Languages.IDs(), r.PackageManagers.IDs(), r.ProjectTypes.IDs()} {
		for _, id := range group {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Strings(out)
	return out
}

// MarkerFiles maps every marker file name declared by a registered package
// manager to the id of the language that owns it. Managers that declare no
// markers are skipped. The architecture test uses it to keep file-name
// conventions inside their language's tree.
func (r Registries) MarkerFiles() map[string]string {
	out := map[string]string{}
	for _, pm := range r.PackageManagers.List() {
		m, ok := pm.(interface{ Markers() []string })
		if !ok {
			continue
		}
		for _, f := range m.Markers() {
			out[f] = pm.Language()
		}
	}
	return out
}
