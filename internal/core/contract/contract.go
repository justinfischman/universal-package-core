// Package contract holds reusable test suites that every registered module must
// pass. Phase 2 and later call RunAll with register.Default(), so a new module
// is checked the moment it is registered, with no test edits.
package contract

import (
	"context"
	"errors"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// RunAll runs the suite for every module in the registries and checks
// cross-module references.
func RunAll(t *testing.T, regs core.Registries) {
	t.Helper()
	for _, l := range regs.Languages.List() {
		t.Run("language/"+l.ID(), func(t *testing.T) { RunLanguageSuite(t, l, regs) })
	}
	for _, pm := range regs.PackageManagers.List() {
		t.Run("pm/"+pm.ID(), func(t *testing.T) {
			RunPackageManagerSuite(t, pm)
			if _, ok := regs.Languages.Get(pm.Language()); !ok {
				t.Errorf("%s belongs to unregistered language %q", pm.ID(), pm.Language())
			}
		})
	}
	for _, pt := range regs.ProjectTypes.List() {
		t.Run("projecttype/"+pt.ID(), func(t *testing.T) { RunProjectTypeSuite(t, pt) })
	}
}

// RunPackageManagerSuite checks capability parity and identity.
func RunPackageManagerSuite(t *testing.T, pm core.PackageManager) {
	t.Helper()
	id := pm.ID()
	if id == "" {
		t.Error("package manager id is empty")
	}
	defer func() {
		if pm.ID() != id {
			t.Errorf("id changed during the suite: %q -> %q", id, pm.ID())
		}
	}()
	if pm.Kind() != core.KindPackageManager {
		t.Errorf("kind = %q", pm.Kind())
	}
	if pm.Language() == "" {
		t.Error("package manager must belong to a language")
	}
	tool := pm.Tool()
	if tool.Binary == "" || tool.InstallHint == "" {
		t.Errorf("tool info incomplete: %+v", tool)
	}

	caps := pm.Capabilities()
	if missing := caps.Missing(); len(missing) > 0 {
		t.Fatalf("%s does not declare capabilities %v; declare each as native, filled or unsupported", pm.ID(), missing)
	}
	for _, c := range core.AllCapabilities {
		s := caps[c]
		if err := s.Validate(); err != nil {
			t.Errorf("%s/%s: %v", pm.ID(), c, err)
		}
		if s.Level != core.LevelUnsupported {
			continue
		}
		// Unsupported capabilities are safe to execute: they must fail loudly.
		_, err := pm.Execute(context.Background(), c, core.Request{})
		var ue *core.UnsupportedError
		if !errors.Is(err, core.ErrUnsupported) || !errors.As(err, &ue) {
			t.Errorf("%s/%s: want UnsupportedError, got %v", pm.ID(), c, err)
			continue
		}
		if ue.PackageManager != pm.ID() || ue.Capability != c {
			t.Errorf("%s/%s: error names %s/%s", pm.ID(), c, ue.PackageManager, ue.Capability)
		}
	}
}

// RunLanguageSuite checks the language's managers exist and belong to it.
func RunLanguageSuite(t *testing.T, l core.Language, regs core.Registries) {
	t.Helper()
	if l.ID() == "" {
		t.Error("language id is empty")
	}
	if l.Kind() != core.KindLanguage {
		t.Errorf("kind = %q", l.Kind())
	}
	pms := l.PackageManagers()
	if len(pms) == 0 {
		t.Fatalf("%s lists no package managers", l.ID())
	}
	isListed := false
	for _, id := range pms {
		if id == l.DefaultPackageManager() {
			isListed = true
		}
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Errorf("%s lists unregistered package manager %q", l.ID(), id)
			continue
		}
		if pm.Language() != l.ID() {
			t.Errorf("%s lists %q, which belongs to %q", l.ID(), id, pm.Language())
		}
	}
	if !isListed {
		t.Errorf("%s default %q is not in its manager list %v", l.ID(), l.DefaultPackageManager(), pms)
	}
}

// RunProjectTypeSuite checks identity.
func RunProjectTypeSuite(t *testing.T, p core.ProjectType) {
	t.Helper()
	if p.ID() == "" {
		t.Error("project type id is empty")
	}
	if p.Kind() != core.KindProjectType {
		t.Errorf("kind = %q", p.Kind())
	}
}
