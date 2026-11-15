package app_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
	"github.com/justinfischman/universal-package-core/internal/projects/app"
)

// templatedLanguage is a fake language that provides app templates.
type templatedLanguage struct {
	coretest.Language
	files fs.FS
}

var _ core.Templater = templatedLanguage{}

func (l templatedLanguage) Templates(projectType string) (fs.FS, bool) {
	return l.files, l.files != nil && projectType == app.ID
}

var appTemplates = fstest.MapFS{
	"server.txt":     {Data: []byte("serve")},
	"APP.md.tmpl":    {Data: []byte("# {{.Name}}\n")},
	"sub/.gitignore": {Data: []byte("x\n")},
}

// setup returns the app type over registries whose "alpha" language has app
// templates, the fake manager (which creates <name>/manifest.txt where it is
// asked to initialise) and an empty base directory.
func setup(t *testing.T, files fs.FS) (*app.Type, *coretest.PackageManager, string) {
	t.Helper()
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	if err := regs.Languages.Register(templatedLanguage{Language: coretest.Language{LangID: "tpl", PMs: []string{"tpl-one"}, Marker: "t"}, files: files}); err != nil {
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
		dir := filepath.Join(req.Dir, req.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "manifest.txt"), []byte("native"), 0o644)
	}
	return app.New(regs, fsx.OS{}), pm, t.TempDir()
}

func request(pm core.PackageManager, base, name string, files fs.FS) core.ScaffoldRequest {
	return core.ScaffoldRequest{
		Request: core.Request{Dir: base}, Name: name, Language: "tpl", PackageManager: "tpl-one", Manager: pm, Templates: files,
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		names = append(names, e.Name())
	}
	return names
}

func read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestScaffoldGolden(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	res, err := st.Scaffold(context.Background(), request(pm, base, "demo", appTemplates))
	if err != nil {
		t.Fatal(err)
	}
	if got := entries(t, base); len(got) != 1 || got[0] != "demo" {
		t.Fatalf("base must hold only the app (no staging left): %v", got)
	}
	dest := filepath.Join(base, "demo")
	golden := map[string]string{
		"manifest.txt":   "native",
		"server.txt":     "serve",
		"APP.md":         "# demo\n",
		"sub/.gitignore": "x\n",
	}
	for rel, want := range golden {
		if got := read(t, filepath.Join(dest, filepath.FromSlash(rel))); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	data, _ := res.Data.(map[string]string)
	if data["path"] != dest || data["name"] != "demo" {
		t.Errorf("data = %v", res.Data)
	}
}

func TestScaffoldAsksManagerForAnAppInAStage(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	var got core.Request
	inner := pm.Hook
	pm.Hook = func(c core.Capability, req core.Request) error {
		got = req
		return inner(c, req)
	}
	if _, err := st.Scaffold(context.Background(), request(pm, base, "demo", appTemplates)); err != nil {
		t.Fatal(err)
	}
	if got.ProjectType != app.ID || got.Name != "demo" {
		t.Errorf("init request = %+v", got)
	}
	if got.Dir == base || filepath.Dir(got.Dir) != base {
		t.Errorf("manager must work in a stage inside %s, got %s", base, got.Dir)
	}
}

func TestScaffoldNeverOverwritesManagerOutput(t *testing.T) {
	clash := fstest.MapFS{"manifest.txt": {Data: []byte("template")}}
	st, pm, base := setup(t, clash)
	_, err := st.Scaffold(context.Background(), request(pm, base, "demo", clash))
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("a failed scaffold must leave nothing behind: %v", got)
	}
}

func TestScaffoldRollsBackWhenManagerFails(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	boom := errors.New("boom")
	pm.Hook = func(core.Capability, core.Request) error { return boom }
	if _, err := st.Scaffold(context.Background(), request(pm, base, "demo", appTemplates)); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("leftovers: %v", got)
	}
}

func TestScaffoldRequiresTheManagerToCreateTheAppDirectory(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	pm.Hook = func(core.Capability, core.Request) error { return nil }
	if _, err := st.Scaffold(context.Background(), request(pm, base, "demo", appTemplates)); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("leftovers: %v", got)
	}
}

func TestScaffoldRefusesExistingDestination(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	if err := os.Mkdir(filepath.Join(base, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Scaffold(context.Background(), request(pm, base, "demo", appTemplates)); !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
	if got := entries(t, filepath.Join(base, "demo")); len(got) != 0 {
		t.Errorf("existing directory was touched: %v", got)
	}
}

func TestScaffoldDryRunChangesNothing(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	var calls []core.Request
	pm.Hook = func(_ core.Capability, req core.Request) error { calls = append(calls, req); return nil }
	req := request(pm, base, "demo", appTemplates)
	req.DryRun = true
	res, err := st.Scaffold(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("dry run wrote: %v", got)
	}
	if len(calls) != 1 || !calls[0].DryRun || calls[0].Dir != base {
		t.Errorf("manager calls = %+v", calls)
	}
	if !strings.Contains(res.Summary, "dry run") {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestScaffoldValidatesInput(t *testing.T) {
	st, pm, base := setup(t, appTemplates)
	cases := map[string]func(*core.ScaffoldRequest){
		"empty name":      func(r *core.ScaffoldRequest) { r.Name = "" },
		"traversal":       func(r *core.ScaffoldRequest) { r.Name = "../x" },
		"leading dash":    func(r *core.ScaffoldRequest) { r.Name = "-x" },
		"no dir":          func(r *core.ScaffoldRequest) { r.Dir = "" },
		"no manager":      func(r *core.ScaffoldRequest) { r.Manager = nil },
		"no templates":    func(r *core.ScaffoldRequest) { r.Templates = nil },
		"unknown stack":   func(r *core.ScaffoldRequest) { r.Language, r.PackageManager = "nope", "nope" },
		"foreign manager": func(r *core.ScaffoldRequest) { r.PackageManager = "alpha-one" },
	}
	for name, mutate := range cases {
		req := request(pm, base, "demo", appTemplates)
		mutate(&req)
		if _, err := st.Scaffold(context.Background(), req); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if got := entries(t, base); len(got) != 0 {
		t.Errorf("invalid requests left files: %v", got)
	}
}

func TestSupportsNeedsTemplatesAndInit(t *testing.T) {
	st, _, _ := setup(t, appTemplates)
	cases := []struct {
		language, pm string
		want         bool
	}{
		{"tpl", "tpl-one", true},
		{"tpl", "", true},
		{"tpl", "alpha-one", false},
		{"alpha", "alpha-one", false}, // language without a Templater
		{"nope", "", false},
	}
	for _, c := range cases {
		if got := st.Supports(c.language, c.pm); got != c.want {
			t.Errorf("Supports(%q, %q) = %v, want %v", c.language, c.pm, got, c.want)
		}
	}
	noTemplates, _, _ := setup(t, nil)
	if noTemplates.Supports("tpl", "tpl-one") {
		t.Error("a language without app templates must not support app")
	}
}

func TestSupportsRefusesAManagerWithoutInit(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	noInit, err := coretest.NewPackageManager("tpl-one", "tpl", core.CapInit)
	if err != nil {
		t.Fatal(err)
	}
	lang := templatedLanguage{Language: coretest.Language{LangID: "tpl", PMs: []string{"tpl-one"}, Marker: "t"}, files: appTemplates}
	if err := regs.Languages.Register(lang); err != nil {
		t.Fatal(err)
	}
	if err := regs.PackageManagers.Register(noInit); err != nil {
		t.Fatal(err)
	}
	if app.New(regs, fsx.OS{}).Supports("tpl", "tpl-one") {
		t.Error("a manager that cannot init must not support app")
	}
}

func TestProjectTypeContract(t *testing.T) {
	st, _, _ := setup(t, appTemplates)
	contract.RunProjectTypeSuite(t, st)
}
