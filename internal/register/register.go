// Package register is the composition root: the only place that imports
// concrete languages, package managers and project types. Adding one is one
// import and one line here (ADR-0002). Nothing registers itself via init().
package register

import (
	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/projects/app"
	"github.com/justinfischman/universal-package-core/internal/projects/packagetype"
	"github.com/justinfischman/universal-package-core/internal/projects/script"
	"github.com/justinfischman/universal-package-core/internal/stacks/compose"
	"github.com/justinfischman/universal-package-core/internal/stacks/just"
	"github.com/justinfischman/universal-package-core/internal/stacks/node"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/npm"
	"github.com/justinfischman/universal-package-core/internal/stacks/node/yarn"
	"github.com/justinfischman/universal-package-core/internal/stacks/python"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/pip"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/poetry"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/uv"
)

// Default builds the registries for the real binary.
func Default() (core.Registries, error) {
	regs := core.NewRegistries()
	manager, err := uv.New(fsx.OS{})
	if err != nil {
		return regs, err
	}
	pipManager, err := pip.New(fsx.OS{})
	if err != nil {
		return regs, err
	}
	poetryManager, err := poetry.New(fsx.OS{})
	if err != nil {
		return regs, err
	}
	npmManager, err := npm.New(fsx.OS{})
	if err != nil {
		return regs, err
	}
	yarnManager, err := yarn.New(fsx.OS{})
	if err != nil {
		return regs, err
	}
	if err := regs.Languages.Register(python.New()); err != nil {
		return regs, err
	}
	if err := regs.Languages.Register(node.New()); err != nil {
		return regs, err
	}
	if err := regs.PackageManagers.Register(manager); err != nil {
		return regs, err
	}
	if err := regs.PackageManagers.Register(pipManager); err != nil {
		return regs, err
	}
	if err := regs.PackageManagers.Register(poetryManager); err != nil {
		return regs, err
	}
	if err := regs.PackageManagers.Register(npmManager); err != nil {
		return regs, err
	}
	if err := regs.PackageManagers.Register(yarnManager); err != nil {
		return regs, err
	}
	if err := regs.ProjectTypes.Register(app.New(regs, fsx.OS{})); err != nil {
		return regs, err
	}
	if err := regs.ProjectTypes.Register(script.New(regs, fsx.OS{})); err != nil {
		return regs, err
	}
	if err := regs.ProjectTypes.Register(packagetype.New(regs, fsx.OS{})); err != nil {
		return regs, err
	}
	regs.RunSources = []core.RunSource{just.New(execx.Ambient{})}
	regs.UpProviders = []core.UpProvider{compose.New(execx.Ambient{})}
	regs.DefaultLanguage = python.ID
	return regs, nil
}
