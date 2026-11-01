package cli

import (
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// switchNote reminds the user what a manager switch does not do.
const switchNote = "upc does not convert manifests or lockfiles; the new manager's own commands own them. Run `upc doctor` to check the project."

// checkManagerSwitch refuses a manager of another language than the project's
// current one (core.ErrIncompatible); unknown ids are left to config.Validate.
// The current language is the configured one, else the language of the
// configured manager, else the one the project's files show. It returns the
// note to print when the effective manager actually changes.
func checkManagerSwitch(s *Session, merged config.Resolved, candidate config.Config, key string) (string, error) {
	canonical := config.CanonicalKey(key)
	if canonical != config.KeyPackageManager && canonical != config.KeyPackageManagers {
		return "", nil
	}
	current := currentLanguage(s, merged)
	ids := candidate.PackageManagers
	if canonical == config.KeyPackageManager {
		ids = []string{candidate.PackageManager}
	}
	for _, id := range ids {
		pm, ok := s.Registries.PackageManagers.Get(id)
		if ok && current != "" && pm.Language() != current {
			return "", fmt.Errorf("%w: package manager %q belongs to language %q, not %q; switching managers is only allowed within a language",
				core.ErrIncompatible, id, pm.Language(), current)
		}
	}
	if canonical == config.KeyPackageManager && merged.PackageManager != "" && merged.PackageManager != candidate.PackageManager {
		return switchNote, nil
	}
	return "", nil
}

// currentLanguage is the language the project is in, or "" when nothing says.
func currentLanguage(s *Session, merged config.Resolved) string {
	if merged.Language != "" {
		return merged.Language
	}
	if pm, ok := s.Registries.PackageManagers.Get(merged.PackageManager); ok {
		return pm.Language()
	}
	if s.Global || s.Store.ProjectRoot == "" {
		return ""
	}
	dirFS := s.FS.DirFS(s.Store.ProjectRoot)
	var found []core.PackageManager
	for _, pm := range s.Registries.PackageManagers.List() {
		if pm.Detect(dirFS) {
			found = append(found, pm)
		}
	}
	if found = highestRanked(found, dirFS); len(found) > 0 {
		return found[0].Language()
	}
	return ""
}
