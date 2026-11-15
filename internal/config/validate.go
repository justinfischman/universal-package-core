package config

import (
	"fmt"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/envs"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var secretKeys = []string{"secret", "password", "token", "key", "api"}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range secretKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// Validate checks a config against the registries: every named module must
// exist, and the package manager must belong to the language. Switching to a
// manager of another language is therefore refused with core.ErrIncompatible.
func Validate(c Config, regs core.Registries) error {
	// Reject secret keys in committed config
	for k := range map[string]interface{}{
		c.Language: nil, c.PackageManager: nil, c.ProjectType: nil,
		c.DefaultEnvironment: nil, c.ActiveEnvironment: nil,
	} {
		if isSecretKey(k) {
			return fmt.Errorf("%w: config contains secret-looking key %q; move it to local.toml", core.ErrInvalidRequest, k)
		}
	}
	for name, env := range c.Environments {
		if isSecretKey(name) {
			return fmt.Errorf("%w: environment name %q looks like a secret; move it to local.toml", core.ErrInvalidRequest, name)
		}
		if isSecretKey(env.File) {
			return fmt.Errorf("%w: environment file %q looks like a secret; move it to local.toml", core.ErrInvalidRequest, env.File)
		}
	}
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
	for _, id := range c.PackageManagers {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			return fmt.Errorf("%w: unknown package manager %q in package_managers (known: %v)", core.ErrInvalidRequest, id, regs.PackageManagers.IDs())
		}
		if c.Language != "" && pm.Language() != c.Language {
			return fmt.Errorf("%w: package manager %q belongs to language %q, not %q",
				core.ErrIncompatible, id, pm.Language(), c.Language)
		}
	}
	if c.ProjectType != "" {
		if _, ok := regs.ProjectTypes.Get(c.ProjectType); !ok {
			return fmt.Errorf("%w: unknown project type %q (known: %v)", core.ErrInvalidRequest, c.ProjectType, regs.ProjectTypes.IDs())
		}
	}
	if err := ValidateApps(c.Apps); err != nil {
		return err
	}
	return validateEnvironments(c)
}

// validateEnvironments checks names and file paths without touching the disk:
// names must be plain words and files must stay inside the project.
func validateEnvironments(c Config) error {
	for name, env := range c.Environments {
		if err := envs.ValidName(name); err != nil {
			return fmt.Errorf("%w: environments: %v", core.ErrInvalidRequest, err)
		}
		if env.File != "" {
			if err := envs.ValidFile(env.File); err != nil {
				return fmt.Errorf("%w: environments.%s: %v", core.ErrInvalidRequest, name, err)
			}
		}
	}
	for key, name := range map[string]string{"default_environment": c.DefaultEnvironment, "active_environment": c.ActiveEnvironment} {
		if name == "" {
			continue
		}
		if err := envs.ValidName(name); err != nil {
			return fmt.Errorf("%w: %s: %v", core.ErrInvalidRequest, key, err)
		}
	}
	return nil
}
