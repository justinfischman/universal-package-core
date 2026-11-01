// Package script is the "script" project type: a single runnable file with its
// dependencies declared inline. It is language-agnostic: the chosen package
// manager creates the script natively (ADR-0005), and this package adds only
// neutral extras. It never names a language or manager.
package script

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/render"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// ID is the project type id.
const ID = "script"

//go:embed templates
var templates embed.FS

// namePattern restricts names to something safe as a directory and file name.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Type implements core.ProjectType.
type Type struct {
	regs core.Registries
	fsys fsx.FS
}

var _ core.ProjectType = (*Type)(nil)

// New builds the script type. It queries regs, never names modules, to decide
// which language and manager pairs it supports.
func New(regs core.Registries, fsys fsx.FS) *Type { return &Type{regs: regs, fsys: fsys} }

// ID implements core.Module.
func (*Type) ID() string { return ID }

// Kind implements core.Module.
func (*Type) Kind() core.Kind { return core.KindProjectType }

// Supports implements core.ProjectType: the manager must belong to the
// language and be able to initialise a project (init is not unsupported). An
// empty manager means the language's default.
func (t *Type) Supports(language, packageManager string) bool {
	if packageManager == "" {
		lang, ok := t.regs.Languages.Get(language)
		if !ok {
			return false
		}
		packageManager = lang.DefaultPackageManager()
	}
	pm, ok := t.regs.PackageManagers.Get(packageManager)
	if !ok || pm.Language() != language {
		return false
	}
	return pm.Capabilities()[core.CapInit].Level != core.LevelUnsupported
}

// templateData is the fixed variable set for templates (ADR-0005).
type templateData struct {
	Name     string
	Module   string
	Language string
	PM       string
	Year     int
}

// Scaffold implements core.ProjectType. The project is built in a staging
// directory next to the destination and moved into place only when complete, so
// a failure leaves nothing behind.
func (t *Type) Scaffold(ctx context.Context, req core.ScaffoldRequest) (core.Result, error) {
	if err := t.validate(req); err != nil {
		return core.Result{}, err
	}
	dest := filepath.Join(req.Dir, req.Name)
	if _, err := t.fsys.Stat(dest); err == nil {
		return core.Result{}, fmt.Errorf("%w: %s already exists", core.ErrInvalidRequest, dest)
	}
	initReq := core.Request{Script: req.Name, ProjectType: ID, DryRun: req.DryRun, Yes: req.Yes}

	if req.DryRun {
		initReq.Dir = dest
		res, err := req.Manager.Execute(ctx, core.CapInit, initReq)
		if err != nil {
			return core.Result{}, err
		}
		return core.Result{Summary: fmt.Sprintf("dry run, nothing changed: would create script %s in %s", req.Name, dest), Commands: res.Commands}, nil
	}

	if err := t.fsys.MkdirAll(req.Dir, 0o755); err != nil {
		return core.Result{}, fmt.Errorf("create %s: %w", req.Dir, err)
	}
	stage, err := t.fsys.MkdirTemp(req.Dir, ".upc-new-*")
	if err != nil {
		return core.Result{}, fmt.Errorf("stage new script: %w", err)
	}
	done := false
	defer func() {
		if !done {
			_ = t.fsys.RemoveAll(stage)
		}
	}()

	initReq.Dir = stage
	res, err := req.Manager.Execute(ctx, core.CapInit, initReq)
	if err != nil {
		return core.Result{}, err
	}
	if err := t.renderExtras(stage, req); err != nil {
		return core.Result{}, err
	}
	if err := t.fsys.Rename(stage, dest); err != nil {
		return core.Result{}, fmt.Errorf("move script into place: %w", err)
	}
	done = true
	return core.Result{
		Summary:  fmt.Sprintf("created script %s in %s", req.Name, dest),
		Data:     map[string]string{"path": dest, "name": req.Name},
		Commands: res.Commands,
	}, nil
}

func (t *Type) validate(req core.ScaffoldRequest) error {
	switch {
	case !namePattern.MatchString(req.Name):
		return fmt.Errorf("%w: %q is not a valid script name (letters, digits, '.', '_' and '-', not starting with a symbol)", core.ErrInvalidRequest, req.Name)
	case req.Dir == "":
		return fmt.Errorf("%w: no destination directory", core.ErrInvalidRequest)
	case req.Manager == nil:
		return fmt.Errorf("%w: no package manager to create the script with", core.ErrInvalidRequest)
	case !t.Supports(req.Language, req.PackageManager):
		return fmt.Errorf("%w: %s/%s cannot create scripts", core.ErrIncompatible, req.Language, req.PackageManager)
	}
	return nil
}

// renderExtras writes the neutral README, then the language's own files.
func (t *Type) renderExtras(stage string, req core.ScaffoldRequest) error {
	data := templateData{
		Name:     req.Name,
		Module:   moduleName(req.Name),
		Language: req.Language,
		PM:       req.PackageManager,
		Year:     time.Now().Year(),
	}
	own, err := fs.Sub(templates, "templates")
	if err != nil {
		return err
	}
	if _, err := render.Render(t.fsys, own, stage, data, render.Options{}); err != nil {
		return err
	}
	if req.Templates == nil {
		return nil
	}
	_, err = render.Render(t.fsys, req.Templates, stage, data, render.Options{})
	return err
}

var nonIdent = regexp.MustCompile(`[^a-z0-9_]+`)

// moduleName turns a script name into an identifier-safe form.
func moduleName(name string) string {
	return strings.Trim(nonIdent.ReplaceAllString(strings.ToLower(name), "_"), "_")
}
