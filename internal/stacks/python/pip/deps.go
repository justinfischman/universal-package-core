package pip

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/components/invoke"
	"github.com/justinfischman/universal-package-core/internal/components/pkgreport"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/python/shared"
)

// install routes to `python -m pip install -r requirements.txt` in .venv,
// creating the environment first when it is missing.
func (p *PIP) install(ctx context.Context, req core.Request) (core.Result, error) {
	groups := req.AllGroups()
	if len(groups) == 0 {
		groups = []string{""}
	}
	var files []string
	for _, g := range groups {
		file, err := requirementsFile(g)
		if err != nil {
			return core.Result{}, err
		}
		files = append(files, file)
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	args := []string{"install"}
	for _, file := range files {
		if _, err := fs.Stat(s.fsys, file); err != nil {
			return core.Result{}, fmt.Errorf("%w: %s not found in %s; run `upc init` or `upc add`", core.ErrInvalidRequest, file, s.dir)
		}
		args = append(args, "-r", file)
	}
	py, err := s.ensureVenv()
	if err != nil {
		return core.Result{}, err
	}
	if err := s.pip(true, py, invoke.Concat(args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	return s.Result("dependencies installed", "install dependencies", nil), nil
}

// add is filled: it installs the packages, then records each one pinned to the
// version pip installed in requirements.txt (requirements-dev.txt for the dev
// group). The file is written once, after every command has succeeded, so a
// failure leaves it unchanged.
func (p *PIP) add(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	file, err := requirementsFile(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	py, err := s.ensureVenv()
	if err != nil {
		return core.Result{}, err
	}
	if err := s.pip(true, py, invoke.Concat([]string{"install"}, req.Packages, req.Args)...); err != nil {
		return core.Result{}, err
	}
	installed, err := s.freeze(py)
	if err != nil {
		return core.Result{}, err
	}
	err = s.edit(file, func(r *requirements) {
		for _, raw := range req.Packages {
			sp, ok := parseSpec(raw)
			if !ok {
				r.set("", strings.TrimSpace(raw))
				continue
			}
			line := strings.TrimSpace(raw)
			if v, found := installed[sp.key]; found {
				line = sp.pinned(v)
			}
			r.set(sp.key, line)
		}
	})
	if err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	return s.Result("added "+list, "add "+list, nil), nil
}

// remove is filled: pip uninstall, then drop the packages from the file.
func (p *PIP) remove(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.RequirePackages(); err != nil {
		return core.Result{}, err
	}
	file, err := requirementsFile(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	keys := map[string]bool{}
	for _, raw := range req.Packages {
		sp, ok := parseSpec(raw)
		if !ok {
			return core.Result{}, fmt.Errorf("%w: %q is not a package name", core.ErrInvalidRequest, raw)
		}
		keys[sp.key] = true
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	py, err := s.existingVenv()
	if err != nil {
		return core.Result{}, err
	}
	names := make([]string, 0, len(req.Packages))
	for _, raw := range req.Packages {
		sp, _ := parseSpec(raw)
		names = append(names, sp.name)
	}
	if err := s.pip(true, py, invoke.Concat([]string{"uninstall", "--yes"}, names, req.Args)...); err != nil {
		return core.Result{}, err
	}
	if err := s.edit(file, func(r *requirements) { r.remove(keys) }); err != nil {
		return core.Result{}, err
	}
	list := strings.Join(req.Packages, " ")
	return s.Result("removed "+list, "remove "+list, nil), nil
}

// update is filled: pip install --upgrade (the named packages, or everything
// in the file), then move the file's pins to the versions now installed.
func (p *PIP) update(ctx context.Context, req core.Request) (core.Result, error) {
	if err := req.ValidatePackages(); err != nil {
		return core.Result{}, err
	}
	file, err := requirementsFile(req.Group)
	if err != nil {
		return core.Result{}, err
	}
	only := map[string]bool{}
	args := []string{"install", "--upgrade"}
	if len(req.Packages) == 0 {
		args = append(args, "-r", file)
	}
	for _, raw := range req.Packages {
		sp, ok := parseSpec(raw)
		if !ok {
			return core.Result{}, fmt.Errorf("%w: %q is not a package name", core.ErrInvalidRequest, raw)
		}
		only[sp.key] = true
		args = append(args, sp.name+sp.extras)
	}
	s, err := p.begin(ctx, req)
	if err != nil {
		return core.Result{}, err
	}
	if len(req.Packages) == 0 {
		if _, err := fs.Stat(s.fsys, file); err != nil {
			return core.Result{}, fmt.Errorf("%w: %s not found in %s", core.ErrInvalidRequest, file, s.dir)
		}
	}
	py, err := s.ensureVenv()
	if err != nil {
		return core.Result{}, err
	}
	if err := s.pip(true, py, invoke.Concat(args, req.Args)...); err != nil {
		return core.Result{}, err
	}
	installed, err := s.freeze(py)
	if err != nil {
		return core.Result{}, err
	}
	if err := s.edit(file, func(r *requirements) { r.repin(installed, only) }); err != nil {
		return core.Result{}, err
	}
	return s.Result("dependencies updated", "update dependencies", nil), nil
}

// freeze reads the versions installed in the environment.
func (s *session) freeze(py string) (map[string]string, error) {
	out, err := s.RunProgram(false, py, "-m", "pip", "freeze")
	if err != nil {
		return nil, err
	}
	return parseFreeze(out.Stdout), nil
}

// edit applies change to a requirements file and writes it atomically, only
// when the content changes. A dry run and a missing file with nothing to add
// write nothing.
func (s *session) edit(file string, change func(*requirements)) error {
	data, err := s.p.fsys.ReadFile(s.path(file))
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return fmt.Errorf("read %s: %w", file, err)
	}
	r := parseRequirements(data)
	change(r)
	out := r.bytes()
	if s.Req.DryRun || (missing && len(out) == 0) || string(out) == string(data) {
		return nil
	}
	if err := s.p.fsys.WriteFileAtomic(s.path(file), out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return nil
}

// list is native: `python -m pip list --format json` in .venv.
func (p *PIP) list(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, s, err := p.inventory(ctx, req, false)
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return s.Result("", "list dependencies", nil), nil
	}
	if pkgs == nil {
		pkgs = []core.Package{}
	}
	summary := pkgreport.Summary(pkgs)
	if summary == "" {
		summary = "no dependencies"
	}
	return s.Result(summary, "", pkgs), nil
}

// outdated is native: `python -m pip list --outdated --format json`.
func (p *PIP) outdated(ctx context.Context, req core.Request) (core.Result, error) {
	pkgs, s, err := p.inventory(ctx, req, true)
	if err != nil {
		return core.Result{}, err
	}
	if req.DryRun {
		return s.Result("", "check for outdated dependencies", nil), nil
	}
	stale, summary := pkgreport.Outdated(pkgs)
	if summary == "" {
		summary = "all dependencies are up to date"
	}
	return s.Result(summary, "", stale), nil
}

func (p *PIP) inventory(ctx context.Context, req core.Request, outdated bool) ([]core.Package, *session, error) {
	s, err := p.begin(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	py, err := s.existingVenv()
	if err != nil {
		return nil, nil, err
	}
	args := []string{"list", "--format", "json"}
	if outdated {
		args = append(args, "--outdated")
	}
	out, err := s.RunProgram(false, py, invoke.Concat([]string{"-m", "pip"}, args, req.Args)...)
	if err != nil {
		return nil, s, err
	}
	if req.DryRun {
		return nil, s, nil
	}
	pkgs, err := shared.ParsePipList(out.Stdout)
	return pkgs, s, err
}
