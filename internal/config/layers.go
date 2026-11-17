package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/core"
)

const (
	// LocalName is the local-only config file.
	LocalName = "local.toml"
)

// ScopeWithLocal extends Scope with Local.
type ScopeWithLocal string

// Config layer scopes, lowest to highest precedence.
const (
	ScopeGlobal  ScopeWithLocal = "global"
	ScopeRoot    ScopeWithLocal = "root"
	ScopeProject ScopeWithLocal = "project"
	ScopeLocal   ScopeWithLocal = "local"
)

// MergeConfig merges two configs: lower wins for scalars, tables merge, lists replace.
func MergeConfig(base, override Config) Config {
	result := base

	// Scalars: override wins if non-empty
	if override.Language != "" {
		result.Language = override.Language
	}
	if override.PackageManager != "" {
		result.PackageManager = override.PackageManager
	}
	if len(override.PackageManagers) > 0 {
		result.PackageManagers = override.PackageManagers
	}
	if override.ProjectType != "" {
		result.ProjectType = override.ProjectType
	}
	if override.DefaultEnvironment != "" {
		result.DefaultEnvironment = override.DefaultEnvironment
	}
	if override.ActiveEnvironment != "" {
		result.ActiveEnvironment = override.ActiveEnvironment
	}

	// Tables merge: environments
	if len(override.Environments) > 0 {
		if result.Environments == nil {
			result.Environments = make(map[string]Environment)
		}
		for k, v := range override.Environments {
			result.Environments[k] = v
		}
	}

	// Lists replace: workspace
	if len(override.Workspace.Members) > 0 {
		result.Workspace = override.Workspace
	}
	result.Apps = mergeApps(result.Apps, override.Apps)

	result.Scripts, result.Up = mergeCommands(base, override)

	return result
}

// mergeCommands merges the configured commands of two layers: scripts merge by
// name (a copy, so the base layer is never mutated), and the up command from
// the override replaces the base one.
func mergeCommands(base, override Config) (map[string][]string, []string) {
	scripts, up := base.Scripts, base.Up
	if len(override.Scripts) > 0 {
		scripts = make(map[string][]string, len(base.Scripts)+len(override.Scripts))
		for k, v := range base.Scripts {
			scripts[k] = v
		}
		for k, v := range override.Scripts {
			scripts[k] = v
		}
	}
	if len(override.Up) > 0 {
		up = override.Up
	}
	return scripts, up
}

// LoadMerged loads config with full layering: global, root, project, local.
func (s Store) LoadMerged() (Resolved, error) {
	// 1. Global
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

	// 2. Root (workspace root) - walk up to find it
	rootDir := s.findWorkspaceRoot()
	if rootDir != "" && rootDir != s.ProjectRoot {
		rootConfig, err := s.loadFromPath(filepath.Join(rootDir, DirName, FileName))
		if err != nil {
			return Resolved{}, err
		}
		res.Config = MergeConfig(res.Config, rootConfig)
		// Track sources for root
		for _, k := range Keys {
			if v, _ := rootConfig.Get(k); v != "" {
				res.Sources[k] = Project // root treated as project-level
			}
		}
		if rootConfig.DefaultEnvironment != "" {
			res.DefaultEnvironment = rootConfig.DefaultEnvironment
		}
		if rootConfig.ActiveEnvironment != "" {
			res.ActiveEnvironment = rootConfig.ActiveEnvironment
		}
		if len(rootConfig.Environments) > 0 {
			if res.Environments == nil {
				res.Environments = make(map[string]Environment)
			}
			for k, v := range rootConfig.Environments {
				res.Environments[k] = v
			}
		}
		if len(rootConfig.Workspace.Members) > 0 {
			res.Workspace = rootConfig.Workspace
		}
	}

	// 3. Project/App (current directory)
	if s.ProjectRoot != "" {
		p, err := s.LoadScope(Project)
		if err != nil {
			return Resolved{}, err
		}
		res.Config = MergeConfig(res.Config, p)
		for _, k := range Keys {
			if v, _ := p.Get(k); v != "" {
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
			if res.Environments == nil {
				res.Environments = make(map[string]Environment)
			}
			for k, v := range p.Environments {
				res.Environments[k] = v
			}
		}
		if len(p.Workspace.Members) > 0 {
			res.Workspace = p.Workspace
		}
	}

	// 4. Local (gitignored)
	if s.ProjectRoot != "" {
		localPath := filepath.Join(s.ProjectRoot, DirName, LocalName)
		localConfig, err := s.loadFromPath(localPath)
		if err != nil {
			return Resolved{}, err
		}
		if !isEmptyConfig(localConfig) {
			res.Config = MergeConfig(res.Config, localConfig)
			for _, k := range Keys {
				if v, _ := localConfig.Get(k); v != "" {
					res.Sources[k] = Project // local is project-scoped
				}
			}
			if localConfig.DefaultEnvironment != "" {
				res.DefaultEnvironment = localConfig.DefaultEnvironment
			}
			if localConfig.ActiveEnvironment != "" {
				res.ActiveEnvironment = localConfig.ActiveEnvironment
			}
			if len(localConfig.Environments) > 0 {
				if res.Environments == nil {
					res.Environments = make(map[string]Environment)
				}
				for k, v := range localConfig.Environments {
					res.Environments[k] = v
				}
			}
			if len(localConfig.Workspace.Members) > 0 {
				res.Workspace = localConfig.Workspace
			}
		}
	}

	return res, nil
}

func (s Store) loadFromPath(path string) (Config, error) {
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

// findWorkspaceRoot walks up from ProjectRoot to find the workspace root.
// A workspace root is a directory containing a manager-native marker or our config directory.
func (s Store) findWorkspaceRoot() string {
	if s.ProjectRoot == "" {
		return ""
	}

	// Walk up looking for a directory that has a marker file or our config dir with workspace config
	for dir := s.ProjectRoot; ; dir = filepath.Dir(dir) {
		if dir == s.GlobalDir {
			break
		}

		// Check if this directory has a .universal-package-core directory with workspace config
		configPath := filepath.Join(dir, DirName, FileName)
		data, err := s.FS.ReadFile(configPath)
		if err == nil {
			var c Config
			dec := toml.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if dec.Decode(&c) == nil && len(c.Workspace.Members) > 0 {
				return dir
			}
		}

		// Check for manager marker files
		dirFS := s.FS.DirFS(dir)
		for _, pm := range s.Registries.PackageManagers.List() {
			if pm.Detect(dirFS) {
				return dir
			}
		}
		for _, lang := range s.Registries.Languages.List() {
			if lang.Detect(dirFS) {
				return dir
			}
		}

		if parent := filepath.Dir(dir); parent == dir {
			break
		}
	}
	return ""
}

// LoadFromPath loads config from the given path.
func (s Store) LoadFromPath(path string) (Config, error) {
	return s.loadFromPath(path)
}

func (s Store) loadFromDir(dir string) (Config, error) {
	path := filepath.Join(dir, DirName, FileName)
	return s.loadFromPath(path)
}

func isEmptyConfig(c Config) bool {
	return c.Language == "" &&
		c.PackageManager == "" &&
		len(c.PackageManagers) == 0 &&
		c.ProjectType == "" &&
		len(c.Environments) == 0 &&
		c.DefaultEnvironment == "" &&
		c.ActiveEnvironment == "" &&
		len(c.Workspace.Members) == 0 &&
		len(c.Apps) == 0 &&
		len(c.Scripts) == 0 &&
		len(c.Up) == 0
}
