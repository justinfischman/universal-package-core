package cli

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// Why a stack was chosen, shown in logs and by doctor.
const (
	sourceConfig   = "config"
	sourceDetected = "detected"
	sourceDefault  = "language default"
)

// errNothingDetected means the directory holds no project upc recognises.
var errNothingDetected = fmt.Errorf("%w: no supported project found", core.ErrInvalidRequest)

// stack is the language and package manager a command acts with.
type stack struct {
	Language core.Language
	PM       core.PackageManager
	Dir      string // project directory
	Source   string
}

// projectDir is the directory commands act in: --project, else the discovered
// project root, else the working directory.
func projectDir(s *Session) (string, error) {
	if s.Global {
		return "", fmt.Errorf("%w: --global applies to `config`, not to project commands", core.ErrInvalidRequest)
	}
	if s.Project != "" {
		return s.Project, nil
	}
	if s.Store.ProjectRoot != "" {
		return s.Store.ProjectRoot, nil
	}
	cwd, err := s.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return cwd, nil
}

// resolveStack picks the stack: explicit config first, then what the directory
// contains, then the detected language's default manager. An incompatible
// config is rejected here.
func resolveStack(s *Session) (stack, error) {
	dir, err := projectDir(s)
	if err != nil {
		return stack{}, err
	}
	return resolveStackIn(s, dir)
}

func resolveStackIn(s *Session, dir string) (stack, error) {
	res, err := s.Store.Load()
	if err != nil {
		return stack{}, err
	}
	if err := config.Validate(res.Config, s.Registries); err != nil {
		return stack{}, err
	}
	regs := s.Registries
	st := stack{Dir: dir}

	switch {
	case res.PackageManager != "":
		st.PM, st.Source = mustPM(regs, res.PackageManager), sourceConfig
	case res.Language != "":
		lang, _ := regs.Languages.Get(res.Language)
		st.PM, st.Source = mustPM(regs, lang.DefaultPackageManager()), sourceConfig
	default:
		if err := detectStack(s, dir, &st); err != nil {
			return stack{}, err
		}
	}
	lang, ok := regs.Languages.Get(st.PM.Language())
	if !ok {
		return stack{}, fmt.Errorf("%w: package manager %q belongs to unregistered language %q", core.ErrInvalidRequest, st.PM.ID(), st.PM.Language())
	}
	st.Language = lang
	return st, nil
}

// mustPM looks up a manager that config validation or a language listing has
// already vouched for; the registry contract guarantees it exists.
func mustPM(regs core.Registries, id string) core.PackageManager {
	pm, _ := regs.PackageManagers.Get(id)
	return pm
}

func detectStack(s *Session, dir string, st *stack) error {
	dirFS := s.FS.DirFS(dir)
	var found []core.PackageManager
	for _, pm := range s.Registries.PackageManagers.List() {
		if pm.Detect(dirFS) {
			found = append(found, pm)
		}
	}
	found = highestRanked(found, dirFS)
	if len(found) > 1 {
		pm, err := askWhich(s, dir, found)
		if err != nil {
			return err
		}
		st.PM, st.Source = pm, sourceDetected
		return nil
	}
	if len(found) == 1 {
		s.Logger.Info("package manager detected", "pm", found[0].ID(), "dir", dir)
		st.PM, st.Source = found[0], sourceDetected
		return nil
	}
	for _, lang := range s.Registries.Languages.List() {
		if lang.Detect(dirFS) {
			st.PM, st.Source = mustPM(s.Registries, lang.DefaultPackageManager()), sourceDefault
			return nil
		}
	}
	return fmt.Errorf("%w in %s; run `upc init` or `upc new`", errNothingDetected, dir)
}

// highestRanked narrows managers whose markers overlap to the ones that match
// the directory most strongly (a lockfile beats a shared manifest). Managers
// that do not rank themselves count as rank zero.
func highestRanked(found []core.PackageManager, fsys fs.FS) []core.PackageManager {
	rank := func(pm core.PackageManager) int {
		if r, ok := pm.(core.DetectRanker); ok {
			return r.DetectRank(fsys)
		}
		return 0
	}
	best := -1
	for _, pm := range found {
		best = max(best, rank(pm))
	}
	var out []core.PackageManager
	for _, pm := range found {
		if rank(pm) == best {
			out = append(out, pm)
		}
	}
	return out
}

// askWhich resolves a tie between equally strong matches: interactively by
// asking, otherwise by failing with the choices listed. The decision is logged.
func askWhich(s *Session, dir string, found []core.PackageManager) (core.PackageManager, error) {
	ids := make([]string, len(found))
	for i, pm := range found {
		ids[i] = pm.ID()
	}
	list := strings.Join(ids, ", ")
	s.Logger.Warn("several package managers match", "dir", dir, "candidates", list)
	if !s.Yes {
		if id, err := s.Prompter.Choose("Several package managers match "+dir+". Which one?", ids); err == nil {
			s.Logger.Info("package manager chosen", "pm", id, "dir", dir)
			return mustPM(s.Registries, id), nil
		}
	}
	return nil, fmt.Errorf("%w: several package managers match %s (%s); choose one with `upc config set package_manager <id>`",
		core.ErrInvalidRequest, dir, list)
}
