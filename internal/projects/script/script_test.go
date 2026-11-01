package script_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
	"github.com/justinfischman/universal-package-core/internal/projects/script"
)

// setup returns the script type over the fake registries, the fake manager
// (which creates "<name>.ext" in the directory it is asked to initialise in),
// and an empty base directory.
func setup(t *testing.T, fsys fsx.FS) (*script.Type, *coretest.PackageManager, string) {
	t.Helper()
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	m, _ := regs.PackageManagers.Get("alpha-one")
	pm := m.(*coretest.PackageManager)
	pm.Hook = func(c core.Capability, req core.Request) error {
		if c != core.CapInit || req.DryRun {
			return nil
		}
		return os.WriteFile(filepath.Join(req.Dir, req.Script+".ext"), []byte("body"), 0o644)
	}
	if fsys == nil {
		fsys = fsx.OS{}
	}
	return script.New(regs, fsys), pm, t.TempDir()
}

func request(pm core.PackageManager, base, name string) core.ScaffoldRequest {
	return core.ScaffoldRequest{
		Request: core.Request{Dir: base}, Name: name, Language: "alpha", PackageManager: "alpha-one", Manager: pm,
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

func TestScaffoldCreatesNativeFileAndNeutralExtras(t *testing.T) {
	st, pm, base := setup(t, nil)
	req := request(pm, base, "hello")
	req.Templates = fstest.MapFS{".gitignore": {Data: []byte("cache/\n")}}

	res, err := st.Scaffold(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(base, "hello")
	got := entries(t, dest)
	want := []string{".gitignore", "README.md", "hello.ext"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("files = %v, want %v", got, want)
	}
	readme, _ := os.ReadFile(filepath.Join(dest, "README.md"))
	if !strings.Contains(string(readme), "# hello") || !strings.Contains(string(readme), "alpha-one") {
		t.Errorf("README = %q", readme)
	}
	if info, _ := os.Stat(dest); info.Mode().Perm() != 0o755 {
		t.Errorf("project dir mode = %v, want 0755", info.Mode().Perm())
	}
	if len(pm.Requests) != 1 || pm.Requests[0].ProjectType != "script" || pm.Requests[0].Script != "hello" {
		t.Errorf("manager request = %+v", pm.Requests)
	}
	if res.Summary == "" {
		t.Error("empty summary")
	}
	// No staging directory remains next to the result.
	if left := entries(t, base); len(left) != 1 {
		t.Errorf("base holds %v, want only the project", left)
	}
}

func TestScaffoldRollsBackWhenManagerFails(t *testing.T) {
	st, pm, base := setup(t, nil)
	pm.Hook = func(core.Capability, core.Request) error { return errors.New("init exploded") }
	_, err := st.Scaffold(context.Background(), request(pm, base, "hello"))
	if err == nil || !strings.Contains(err.Error(), "exploded") {
		t.Fatalf("err = %v", err)
	}
	if left := entries(t, base); len(left) != 0 {
		t.Errorf("failed scaffold left %v behind", left)
	}
}

// failingFS injects a failure at one filesystem step.
type failingFS struct {
	fsx.FS
	failRename bool
	failWrite  string
}

func (f failingFS) Rename(a, b string) error {
	if f.failRename {
		return errors.New("rename refused")
	}
	return f.FS.Rename(a, b)
}

func (f failingFS) WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if f.failWrite != "" && strings.HasSuffix(path, f.failWrite) {
		return errors.New("disk full")
	}
	return f.FS.WriteFileAtomic(path, data, perm)
}

func TestScaffoldRollsBackOnLateFailures(t *testing.T) {
	cases := map[string]failingFS{
		"rename fails":        {FS: fsx.OS{}, failRename: true},
		"extras write fails":  {FS: fsx.OS{}, failWrite: "README.md"},
		"template write fail": {FS: fsx.OS{}, failWrite: ".gitignore"},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			st, pm, base := setup(t, fsys)
			req := request(pm, base, "hello")
			req.Templates = fstest.MapFS{".gitignore": {Data: []byte("x")}}
			if _, err := st.Scaffold(context.Background(), req); err == nil {
				t.Fatal("want an error")
			}
			if left := entries(t, base); len(left) != 0 {
				t.Errorf("failed scaffold left %v behind", left)
			}
		})
	}
}

func TestScaffoldRefusesExistingDestination(t *testing.T) {
	st, pm, base := setup(t, nil)
	if err := os.Mkdir(filepath.Join(base, "hello"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := st.Scaffold(context.Background(), request(pm, base, "hello"))
	if !errors.Is(err, core.ErrInvalidRequest) || len(pm.Calls) != 0 {
		t.Errorf("err = %v, manager calls = %v", err, pm.Calls)
	}
}

func TestScaffoldDryRunChangesNothing(t *testing.T) {
	st, pm, base := setup(t, nil)
	req := request(pm, base, "hello")
	req.DryRun = true
	res, err := st.Scaffold(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if left := entries(t, base); len(left) != 0 {
		t.Errorf("dry run created %v", left)
	}
	if !strings.Contains(res.Summary, "dry run") || len(pm.Requests) != 1 || !pm.Requests[0].DryRun {
		t.Errorf("result %+v, requests %+v", res, pm.Requests)
	}
}

func TestScaffoldValidatesInput(t *testing.T) {
	st, pm, base := setup(t, nil)
	for _, name := range []string{"", "../escape", "a/b", "-flag", ".hidden", "sp ace"} {
		if _, err := st.Scaffold(context.Background(), request(pm, base, name)); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("name %q: err = %v", name, err)
		}
	}
	noManager := request(nil, base, "ok")
	if _, err := st.Scaffold(context.Background(), noManager); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("nil manager: %v", err)
	}
	wrong := request(pm, base, "ok")
	wrong.Language = "beta"
	if _, err := st.Scaffold(context.Background(), wrong); !errors.Is(err, core.ErrIncompatible) {
		t.Errorf("mismatched pair: %v", err)
	}
	if len(pm.Calls) != 0 || len(entries(t, base)) != 0 {
		t.Error("invalid input must not touch the manager or the disk")
	}
}

func TestSupportsQueriesRegistryNotNames(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	noInit, err := coretest.NewPackageManager("gamma-one", "gamma", core.CapInit)
	if err != nil {
		t.Fatal(err)
	}
	if err := regs.Languages.Register(coretest.Language{LangID: "gamma", PMs: []string{"gamma-one"}, Marker: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := regs.PackageManagers.Register(noInit); err != nil {
		t.Fatal(err)
	}
	st := script.New(regs, fsx.OS{})
	cases := []struct {
		language, pm string
		want         bool
	}{
		{"alpha", "alpha-one", true},
		{"alpha", "alpha-two", true},
		{"alpha", "", true},
		{"beta", "beta-one", true},
		{"alpha", "beta-one", false},
		{"alpha", "nope", false},
		{"nope", "", false},
		{"gamma", "gamma-one", false},
	}
	for _, c := range cases {
		if got := st.Supports(c.language, c.pm); got != c.want {
			t.Errorf("Supports(%q, %q) = %v, want %v", c.language, c.pm, got, c.want)
		}
	}
}

func TestProjectTypeContract(t *testing.T) {
	st, _, _ := setup(t, nil)
	contract.RunProjectTypeSuite(t, st)
}
