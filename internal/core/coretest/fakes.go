// Package coretest provides fake modules for testing core, cli and contract
// suites without any concrete language or package manager.
package coretest

import (
	"context"
	"io/fs"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// PackageManager is a fake manager that declares every capability. Capabilities
// listed in Unsupported are declared unsupported; all others are native and
// record their calls.
type PackageManager struct {
	core.BasePackageManager
	Calls []core.Capability
}

// NewPackageManager builds a fake manager for a language.
func NewPackageManager(id, language string, unsupported ...core.Capability) (*PackageManager, error) {
	pm := &PackageManager{
		BasePackageManager: core.NewBasePackageManager(id, language,
			core.Tool{Binary: id, VersionArgs: []string{"--version"}, InstallHint: "install " + id}, id+".marker"),
	}
	skip := map[core.Capability]bool{}
	for _, c := range unsupported {
		skip[c] = true
	}
	for _, c := range core.AllCapabilities {
		var err error
		if skip[c] {
			err = pm.Declare(c, core.Unsupported("fake: not supported"), nil)
		} else {
			c := c
			err = pm.Declare(c, core.Native(), func(context.Context, core.Request) (core.Result, error) {
				pm.Calls = append(pm.Calls, c)
				return core.Result{Summary: id + " " + string(c)}, nil
			})
		}
		if err != nil {
			return nil, err
		}
	}
	return pm, nil
}

// Language is a fake language.
type Language struct {
	LangID string
	PMs    []string
	Marker string
}

// ID implements core.Module.
func (l Language) ID() string { return l.LangID }

// Kind implements core.Module.
func (l Language) Kind() core.Kind { return core.KindLanguage }

// PackageManagers implements core.Language.
func (l Language) PackageManagers() []string { return l.PMs }

// DefaultPackageManager implements core.Language.
func (l Language) DefaultPackageManager() string { return l.PMs[0] }

// Detect implements core.Language.
func (l Language) Detect(fsys fs.FS) bool {
	_, err := fs.Stat(fsys, l.Marker)
	return err == nil
}

// ProjectType is a fake project type that supports every pair.
type ProjectType struct {
	TypeID string
}

// ID implements core.Module.
func (p ProjectType) ID() string { return p.TypeID }

// Kind implements core.Module.
func (p ProjectType) Kind() core.Kind { return core.KindProjectType }

// Supports implements core.ProjectType.
func (p ProjectType) Supports(string, string) bool { return true }

// Scaffold implements core.ProjectType.
func (p ProjectType) Scaffold(_ context.Context, req core.ScaffoldRequest) (core.Result, error) {
	return core.Result{Summary: "scaffolded " + req.Name}, nil
}

// Registries returns a registry set holding one fake language with two
// managers and one fake project type.
func Registries() (core.Registries, error) {
	regs := core.NewRegistries()
	lang := Language{LangID: "alpha", PMs: []string{"alpha-one", "alpha-two"}, Marker: "alpha.marker"}
	other := Language{LangID: "beta", PMs: []string{"beta-one"}, Marker: "beta.marker"}
	for _, l := range []core.Language{lang, other} {
		if err := regs.Languages.Register(l); err != nil {
			return regs, err
		}
	}
	for _, spec := range [][2]string{{"alpha-one", "alpha"}, {"alpha-two", "alpha"}, {"beta-one", "beta"}} {
		pm, err := NewPackageManager(spec[0], spec[1])
		if err != nil {
			return regs, err
		}
		if err := regs.PackageManagers.Register(pm); err != nil {
			return regs, err
		}
	}
	if err := regs.ProjectTypes.Register(ProjectType{TypeID: "gadget"}); err != nil {
		return regs, err
	}
	return regs, nil
}
