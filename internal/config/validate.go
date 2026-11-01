package config

import (
	"fmt"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Validate checks a config against the registries: every named module must
// exist, and the package manager must belong to the language. Switching to a
// manager of another language is therefore refused with core.ErrIncompatible.
func Validate(c Config, regs core.Registries) error {
	if c.Language != "" {
		if _, ok := regs.Languages.Get(c.Language); !ok {
			return fmt.Errorf("%w: unknown language %q (known: %v)", core.ErrInvalidRequest, c.Language, regs.Languages.IDs())
		}
	}
	if c.PackageManager != "" {
		pm, ok := regs.PackageManagers.Get(c.PackageManager)
		if !ok {
			return fmt.Errorf("%w: unknown package manager %q (known: %v)", core.ErrInvalidRequest, c.PackageManager, regs.PackageManagers.IDs())
		}
		if c.Language != "" && pm.Language() != c.Language {
			return fmt.Errorf("%w: package manager %q belongs to language %q, not %q; switching managers is only allowed within a language",
				core.ErrIncompatible, c.PackageManager, pm.Language(), c.Language)
		}
	}
	if c.ProjectType != "" {
		if _, ok := regs.ProjectTypes.Get(c.ProjectType); !ok {
			return fmt.Errorf("%w: unknown project type %q (known: %v)", core.ErrInvalidRequest, c.ProjectType, regs.ProjectTypes.IDs())
		}
	}
	return nil
}
