package app_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
	"github.com/justinfischman/universal-package-core/internal/projects/app"
)

// kindLanguage provides templates per project type key, such as "app-cli".
type kindLanguage struct {
	coretest.Language
	sets map[string]fs.FS
}

var _ core.Templater = kindLanguage{}

func (l kindLanguage) Templates(projectType string) (fs.FS, bool) {
	files, ok := l.sets[projectType]
	return files, ok
}

func kindSetup(t *testing.T, sets map[string]fs.FS) (*app.Type, *coretest.PackageManager, string) {
	t.Helper()
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	lang := kindLanguage{Language: coretest.Language{LangID: "tpl", PMs: []string{"tpl-one"}, Marker: "t"}, sets: sets}
	if err := regs.Languages.Register(lang); err != nil {
		t.Fatal(err)
	}
	pm, err := coretest.NewPackageManager("tpl-one", "tpl")
	if err != nil {
		t.Fatal(err)
	}
	if err := regs.PackageManagers.Register(pm); err != nil {
		t.Fatal(err)
	}
	pm.Hook = func(c core.Capability, req core.Request) error {
		if c != core.CapInit || req.DryRun {
			return nil
		}
		return os.MkdirAll(filepath.Join(req.Dir, req.Name), 0o755)
	}
	return app.New(regs, fsx.OS{}), pm, t.TempDir()
}

func TestScaffoldKindReplacesServiceTemplates(t *testing.T) {
	service := fstest.MapFS{"server.txt": {Data: []byte("serve")}}
	cli := fstest.MapFS{"main.txt": {Data: []byte("cli")}, "APP.md.tmpl": {Data: []byte("# {{.Name}}\n")}}
	typ, pm, base := kindSetup(t, map[string]fs.FS{"app": service, "app-cli": cli})
	req := request(pm, base, "tool", service)
	req.Kind = "cli"
	if _, err := typ.Scaffold(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := entries(t, filepath.Join(base, "tool"))
	if len(got) != 2 || got[0] != "APP.md" || got[1] != "main.txt" {
		t.Errorf("entries = %v, want the cli templates only", got)
	}
}

func TestScaffoldKindErrors(t *testing.T) {
	service := fstest.MapFS{"server.txt": {Data: []byte("serve")}}
	typ, pm, base := kindSetup(t, map[string]fs.FS{"app": service})
	cases := []struct {
		kind string
		want error
	}{
		{"daemon", core.ErrInvalidRequest},
		{"library", core.ErrIncompatible}, // the language has no library templates
	}
	for _, c := range cases {
		req := request(pm, base, "tool", service)
		req.Kind = c.kind
		if _, err := typ.Scaffold(context.Background(), req); !errors.Is(err, c.want) {
			t.Errorf("kind %q: err = %v, want %v", c.kind, err, c.want)
		}
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("a refused kind left %v behind", got)
	}
}
