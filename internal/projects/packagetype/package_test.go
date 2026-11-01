package packagetype_test

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
	"github.com/justinfischman/universal-package-core/internal/projects/packagetype"
)

// setup returns the package type over the fake registries, the fake manager
// (which, like a native init, creates <Dir>/<Name>/<Name>.ext and a README),
// and an empty base directory.
func setup(t *testing.T, fsys fsx.FS) (*packagetype.Type, *coretest.PackageManager, string) {
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
		root := filepath.Join(req.Dir, req.Name)
		if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, "src", req.Name+".ext"), []byte("body"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, "README.md"), []byte("native"), 0o644)
	}
	if fsys == nil {
		fsys = fsx.OS{}
	}
	return packagetype.New(regs, fsys), pm, t.TempDir()
}

func request(pm core.PackageManager, base, name string) core.ScaffoldRequest {
	return core.ScaffoldRequest{
		Request: core.Request{Dir: base}, Name: name, Language: "alpha", PackageManager: "alpha-one", Manager: pm,
	}
}

func tree(t *testing.T, root string) string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(files, ",")
}

func TestProjectTypeContract(t *testing.T) {
	st, _, _ := setup(t, nil)
	contract.RunProjectTypeSuite(t, st)
}

func TestScaffoldKeepsNativeFilesAndAddsExtras(t *testing.T) {
	st, pm, base := setup(t, nil)
	req := request(pm, base, "demo-lib")
	req.Templates = fstest.MapFS{
		".gitignore":           {Data: []byte("cache/\n")},
		"tests/test_x.py.tmpl": {Data: []byte("import {{.Module}}\n")},
	}
	res, err := st.Scaffold(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(base, "demo-lib")
	if got, want := tree(t, dest), ".gitignore,LICENSE,README.md,src/demo-lib.ext,tests/test_x.py"; got != want {
		t.Errorf("tree = %s, want %s", got, want)
	}
	if readme, _ := os.ReadFile(filepath.Join(dest, "README.md")); string(readme) != "native" {
		t.Errorf("README = %q; the manager's own file must win", readme)
	}
	if body, _ := os.ReadFile(filepath.Join(dest, "tests", "test_x.py")); string(body) != "import demo_lib\n" {
		t.Errorf("template = %q", body)
	}
	if len(pm.Requests) != 1 || pm.Requests[0].ProjectType != "package" || pm.Requests[0].Name != "demo-lib" {
		t.Errorf("manager request = %+v", pm.Requests)
	}
	if res.Summary == "" {
		t.Error("empty summary")
	}
	if entries, _ := os.ReadDir(base); len(entries) != 1 {
		t.Errorf("base holds %d entries, want only the project (staging must be gone)", len(entries))
	}
}

func TestScaffoldKeepsAManagerLicense(t *testing.T) {
	st, pm, base := setup(t, nil)
	inner := pm.Hook
	pm.Hook = func(c core.Capability, req core.Request) error {
		if err := inner(c, req); err != nil || req.DryRun {
			return err
		}
		return os.WriteFile(filepath.Join(req.Dir, req.Name, "LICENSE"), []byte("native license"), 0o644)
	}
	if _, err := st.Scaffold(context.Background(), request(pm, base, "demo")); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(filepath.Join(base, "demo", "LICENSE")); string(body) != "native license" {
		t.Errorf("LICENSE = %q", body)
	}
}

func TestScaffoldDryRunWritesNothing(t *testing.T) {
	st, pm, base := setup(t, nil)
	req := request(pm, base, "demo")
	req.DryRun = true
	res, err := st.Scaffold(context.Background(), req)
	if err != nil || !strings.HasPrefix(res.Summary, "dry run") {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if entries, _ := os.ReadDir(base); len(entries) != 0 {
		t.Errorf("dry run left %d entries", len(entries))
	}
	if len(pm.Requests) != 1 || !pm.Requests[0].DryRun || pm.Requests[0].Dir != base {
		t.Errorf("manager request = %+v", pm.Requests)
	}
}

func TestScaffoldRejectsBadRequests(t *testing.T) {
	st, pm, base := setup(t, nil)
	if err := os.Mkdir(filepath.Join(base, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := request(pm, base, "demo")
	other.PackageManager = "beta-one"
	noMgr := request(nil, base, "demo")
	noDir := request(pm, "", "demo")
	for name, tc := range map[string]struct {
		req  core.ScaffoldRequest
		want error
	}{
		"empty name":   {request(pm, base, ""), core.ErrInvalidRequest},
		"flag-like":    {request(pm, base, "-x"), core.ErrInvalidRequest},
		"path in name": {request(pm, base, "a/b"), core.ErrInvalidRequest},
		"exists":       {request(pm, base, "taken"), core.ErrInvalidRequest},
		"no manager":   {noMgr, core.ErrInvalidRequest},
		"no dir":       {noDir, core.ErrInvalidRequest},
		"wrong pair":   {other, core.ErrIncompatible},
	} {
		if _, err := st.Scaffold(context.Background(), tc.req); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if len(pm.Requests) != 0 {
		t.Errorf("manager must not run for a bad request: %+v", pm.Requests)
	}
}

func TestScaffoldRollsBackWhenManagerFailsOrCreatesNothing(t *testing.T) {
	for name, hook := range map[string]func(core.Capability, core.Request) error{
		"manager fails":        func(core.Capability, core.Request) error { return errors.New("init exploded") },
		"manager made nothing": func(core.Capability, core.Request) error { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			st, pm, base := setup(t, nil)
			pm.Hook = hook
			if _, err := st.Scaffold(context.Background(), request(pm, base, "demo")); err == nil {
				t.Fatal("want an error")
			}
			if entries, _ := os.ReadDir(base); len(entries) != 0 {
				t.Errorf("failed scaffold left %d entries behind", len(entries))
			}
		})
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
		"license write fails": {FS: fsx.OS{}, failWrite: "LICENSE"},
		"template write fail": {FS: fsx.OS{}, failWrite: ".gitignore"},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			st, pm, base := setup(t, fsys)
			req := request(pm, base, "demo")
			req.Templates = fstest.MapFS{".gitignore": {Data: []byte("x")}}
			if _, err := st.Scaffold(context.Background(), req); err == nil {
				t.Fatal("want an error")
			}
			if entries, _ := os.ReadDir(base); len(entries) != 0 {
				t.Errorf("failed scaffold left %d entries behind", len(entries))
			}
		})
	}
}
