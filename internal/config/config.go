// Package config holds upc's own choices (language, package manager, project
// type, environment mapping, workspace members) in layered TOML files. It never
// stores dependency state; the package manager owns that (ADR-0004).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

const (
	// DirName is the hidden directory name used for both global and project config.
	DirName = ".universal-package-core"
	// FileName is the config file inside DirName.
	FileName = "config.toml"
	// HomeEnv overrides the global directory location.
	HomeEnv = "UPC_HOME"
)

// Scope is where a config value comes from.
type Scope string

// The scopes. Project overrides Global.
const (
	Global  Scope = "global"
	Project Scope = "project"
)

// Config is the set of upc's own choices.
type Config struct {
	Language       string `toml:"language,omitempty"`
	PackageManager string `toml:"package_manager,omitempty"`
	// PackageManagers lists every manager a project uses when it uses several.
	PackageManagers []string `toml:"package_managers,omitempty"`
	ProjectType     string   `toml:"project_type,omitempty"`
	// Environments maps a name to its env file (ADR-0006). upc never reads the file.
	Environments map[string]Environment `toml:"environments,omitempty"`
	// DefaultEnvironment is the environment used when none was selected.
	DefaultEnvironment string `toml:"default_environment,omitempty"`
	// ActiveEnvironment is the selection made by `upc env use`.
	ActiveEnvironment string `toml:"active_environment,omitempty"`
	// Workspace defines member projects.
	Workspace Workspace `toml:"workspace,omitempty"`
}

// Environment is one declared environment. An empty File means the convention
// file `.env.<name>`.
type Environment struct {
	File   string   `toml:"file,omitempty"`
	Groups []string `toml:"groups,omitempty"`
}

// Workspace defines project members.
type Workspace struct {
	Members []string `toml:"members,omitempty"`
}

// EffectiveEnvironment is the selected environment, else the default, else "".
func (c Config) EffectiveEnvironment() string {
	if c.ActiveEnvironment != "" {
		return c.ActiveEnvironment
	}
	return c.DefaultEnvironment
}

// Keys settable and gettable through `upc config`.
const (
	KeyLanguage       = "language"
	KeyPackageManager = "package_manager"
	// KeyPackageManagers holds a comma-separated list of manager ids.
	KeyPackageManagers = "package_managers"
	KeyProjectType     = "project_type"

	// aliasPackageManager is the short form of KeyPackageManager.
	aliasPackageManager = "pm"
)

// Keys lists the settable keys in display order.
var Keys = []string{KeyLanguage, KeyPackageManager, KeyPackageManagers, KeyProjectType}

// CanonicalKey resolves a key alias (`pm`) to its stored key.
func CanonicalKey(key string) string {
	if key == aliasPackageManager {
		return KeyPackageManager
	}
	return key
}

// Get returns a scalar value by key.
func (c Config) Get(key string) (string, error) {
	switch CanonicalKey(key) {
	case KeyLanguage:
		return c.Language, nil
	case KeyPackageManager:
		return c.PackageManager, nil
	case KeyPackageManagers:
		return strings.Join(c.PackageManagers, ","), nil
	case KeyProjectType:
		return c.ProjectType, nil
	}
	return "", unknownKey(key)
}

// Set assigns a scalar value by key and returns the updated copy.
func (c Config) Set(key, value string) (Config, error) {
	switch CanonicalKey(key) {
	case KeyLanguage:
		c.Language = value
	case KeyPackageManager:
		c.PackageManager = value
	case KeyPackageManagers:
		c.PackageManagers = splitList(value)
	case KeyProjectType:
		c.ProjectType = value
	default:
		return c, unknownKey(key)
	}
	return c, nil
}

// splitList parses a comma-separated list, dropping blanks and repeats.
func splitList(value string) []string {
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func unknownKey(key string) error {
	return fmt.Errorf("%w: unknown config key %q (valid: %v)", core.ErrInvalidRequest, key, Keys)
}

// Resolved is the merged config plus where each scalar key came from.
type Resolved struct {
	Config
	Sources map[string]Scope
}

// GlobalDir returns the global config directory: $UPC_HOME, else ~/.universal-package-core.
func GlobalDir(getenv func(string) string, homeDir func() (string, error)) (string, error) {
	if v := getenv(HomeEnv); v != "" {
		return v, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// DefaultGlobalDir resolves the global directory from the real environment.
func DefaultGlobalDir() (string, error) { return GlobalDir(os.Getenv, os.UserHomeDir) }

// Store reads and writes config files. ProjectRoot may be empty when no project
// is in play, in which case only the global scope is available.
type Store struct {
	FS           fsx.FS
	GlobalDir    string
	ProjectRoot  string
	Registries   core.Registries
}

func (s Store) file(scope Scope) (string, error) {
	switch scope {
	case Global:
		return filepath.Join(s.GlobalDir, FileName), nil
	case Project:
		if s.ProjectRoot == "" {
			return "", fmt.Errorf("%w: no project directory for project-scoped config", core.ErrInvalidRequest)
		}
		return filepath.Join(s.ProjectRoot, DirName, FileName), nil
	}
	return "", fmt.Errorf("%w: unknown scope %q", core.ErrInvalidRequest, scope)
}

// LoadScope reads one scope. A missing file yields an empty Config.
func (s Store) LoadScope(scope Scope) (Config, error) {
	path, err := s.file(scope)
	if err != nil {
		return Config{}, err
	}
	data, err := s.FS.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%w: %s: %v", core.ErrInvalidRequest, path, err)
	}
	return c, nil
}

// Load merges global then project; project wins field by field.
func (s Store) Load() (Resolved, error) {
	g, err := s.LoadScope(Global)
	if err != nil {
		return Resolved{}, err
	}
	res := Resolved{Config: g, Sources: map[string]Scope{}}
	for _, k := range Keys {
		if v, _ := g.Get(k); v != "" {
			res.Sources[k] = Global
		}
	}
	if s.ProjectRoot == "" {
		return res, nil
	}
	p, err := s.LoadScope(Project)
	if err != nil {
		return Resolved{}, err
	}
	for _, k := range Keys {
		if v, _ := p.Get(k); v != "" {
			res.Config, _ = res.Set(k, v)
			res.Sources[k] = Project
		}
	}
	if p.DefaultEnvironment != "" {
		res.DefaultEnvironment = p.DefaultEnvironment
	}
	if p.ActiveEnvironment != "" {
		res.ActiveEnvironment = p.ActiveEnvironment
	}
	if len(p.Environments) > 0 {
		merged := map[string]Environment{}
		for k, v := range g.Environments {
			merged[k] = v
		}
		for k, v := range p.Environments {
			merged[k] = v
		}
		res.Environments = merged
	}
	if len(p.Workspace.Members) > 0 {
		res.Workspace = p.Workspace
	}
	return res, nil
}

// Save writes one scope atomically, creating its directory lazily.
func (s Store) Save(scope Scope, c Config) error {
	path, err := s.file(scope)
	if err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return s.FS.WriteFileAtomic(path, data, 0o644)
}
