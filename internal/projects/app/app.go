// Package app is the "app" project type: a runnable application. It is
// language-agnostic: the chosen package manager creates the project natively
// (ADR-0005) and this package adds the language's own template files on top.
// It never names a language or manager.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/render"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// ID is the project type id.
const ID = "app"

// namePattern restricts names to something safe as a directory and project name.
var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// Type implements core.ProjectType.
type Type struct {
	regs core.Registries
	fsys fsx.FS
}

var _ core.ProjectType = (*Type)(nil)

// New builds the app type. It queries regs, never names modules, to decide
// which language and manager pairs it supports.
func New(regs core.Registries, fsys fsx.FS) *Type { return &Type{regs: regs, fsys: fsys} }

// ID implements core.Module.
func (*Type) ID() string { return ID }

// Kind implements core.Module.
func (*Type) Kind() core.Kind { return core.KindProjectType }

// Supports implements core.ProjectType: the language must provide app
// templates and the manager must be able to initialise an app. A manager that
// reports init as unsupported, or that does not support an app yet, is not
// offered. An empty manager means the language's default.
func (t *Type) Supports(language, packageManager string) bool {
	lang, ok := t.regs.Languages.Get(language)
	if !ok {
		return false
	}
	templater, ok := lang.(core.Templater)
	if !ok {
		return false
	}
	if _, ok := templater.Templates(ID); !ok {
		return false
	}
	if packageManager == "" {
		packageManager = lang.DefaultPackageManager()
	}
	pm, ok := t.regs.PackageManagers.Get(packageManager)
	if !ok || pm.Language() != language {
		return false
	}
	return pm.Capabilities()[core.CapInit].Level != core.LevelUnsupported
}

// templateData is the fixed variable set for app templates.
type templateData struct {
	Name string
}

// Scaffold implements core.ProjectType. The manager initialises the app in a
// staging directory, the language's templates are rendered into it (never over
// manager output: render refuses to overwrite), and the result is moved into
// place only when complete, so a failure leaves nothing behind.
func (t *Type) Scaffold(ctx context.Context, req core.ScaffoldRequest) (core.Result, error) {
	if err := t.validate(req); err != nil {
		return core.Result{}, err
	}
	dest := filepath.Join(req.Dir, req.Name)
	if _, err := t.fsys.Stat(dest); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, dest)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return core.Result{}, err
	}
	initReq := core.Request{Dir: req.Dir, Name: req.Name, ProjectType: ID, DryRun: req.DryRun, Yes: req.Yes}

	if req.DryRun {
		res, err := req.Manager.Execute(ctx, core.CapInit, initReq)
		if err != nil {
			return core.Result{}, err
		}
		return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would create app %s in %s", req.Name, dest), Commands: res.Commands}, nil
	}

	if err := t.fsys.MkdirAll(req.Dir, 0o755); err != nil {
		return core.Result{}, fmt.Errorf("create %s: %w", req.Dir, err)
	}
	stage, err := t.fsys.MkdirTemp(req.Dir, ".upc-new-*")
	if err != nil {
		return core.Result{}, fmt.Errorf("stage new app: %w", err)
	}
	defer func() { _ = t.fsys.RemoveAll(stage) }()

	initReq.Dir = stage
	res, err := req.Manager.Execute(ctx, core.CapInit, initReq)
	if err != nil {
		return core.Result{}, err
	}
	staged := filepath.Join(stage, req.Name)
	if info, err := t.fsys.Stat(staged); err != nil || !info.IsDir() {
		return core.Result{}, fmt.Errorf("%w: the package manager did not create the app directory %q", core.ErrInvalidRequest, req.Name)
	}
	if _, err := render.Render(t.fsys, req.Templates, staged, templateData{Name: req.Name}, render.Options{}); err != nil {
		return core.Result{}, err
	}
	if err := t.fsys.Rename(staged, dest); err != nil {
		return core.Result{}, fmt.Errorf("move app into place: %w", err)
	}
	return core.Result{
		Summary:  fmt.Sprintf("created app %s in %s", req.Name, dest),
		Data:     map[string]string{"path": dest, "name": req.Name},
		Commands: res.Commands,
	}, nil
}

func (t *Type) validate(req core.ScaffoldRequest) error {
	switch {
	case !namePattern.MatchString(req.Name):
		return fmt.Errorf("%w: %q is not a valid app name (a letter, then letters, digits, '_' and '-')", core.ErrInvalidRequest, req.Name)
	case req.Dir == "":
		return fmt.Errorf("%w: no destination directory", core.ErrInvalidRequest)
	case req.Manager == nil:
		return fmt.Errorf("%w: no package manager to create the app with", core.ErrInvalidRequest)
	case req.Templates == nil:
		return fmt.Errorf("%w: %s has no app templates", core.ErrIncompatible, req.Language)
	case !t.Supports(req.Language, req.PackageManager):
		return fmt.Errorf("%w: %s/%s cannot create apps", core.ErrIncompatible, req.Language, req.PackageManager)
	}
	return nil
}
