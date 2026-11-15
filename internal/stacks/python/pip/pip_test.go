package pip

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

const proj = "/proj"

// memFS is an in-memory fsx.FS keyed by full path. failWrite makes every
// atomic write fail, to prove files are left unchanged.
type memFS struct {
	files     map[string]string
	failWrite error
	writes    int
}

var _ fsx.FS = (*memFS)(nil)

func newMem(files map[string]string) *memFS {
	m := &memFS{files: map[string]string{}}
	for name, body := range files {
		m.files[filepath.Join(proj, name)] = body
	}
	return m
}

func (m *memFS) ReadFile(p string) ([]byte, error) {
	if b, ok := m.files[p]; ok {
		return []byte(b), nil
	}
	return nil, fs.ErrNotExist
}

func (m *memFS) WriteFileAtomic(p string, data []byte, _ fs.FileMode) error {
	if m.failWrite != nil {
		return m.failWrite
	}
	m.writes++
	m.files[p] = string(data)
	return nil
}

func (m *memFS) Stat(p string) (fs.FileInfo, error) {
	if _, ok := m.files[p]; ok {
		return fstest.MapFS{"f": {}}.Stat("f")
	}
	return nil, fs.ErrNotExist
}

func (m *memFS) DirFS(dir string) fs.FS {
	out := fstest.MapFS{}
	for p, body := range m.files {
		if rel, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(rel, "..") {
			out[filepath.ToSlash(rel)] = &fstest.MapFile{Data: []byte(body)}
		}
	}
	return out
}

func (m *memFS) MkdirAll(string, fs.FileMode) error       { return nil }
func (m *memFS) Remove(string) error                      { return nil }
func (m *memFS) RemoveAll(string) error                   { return nil }
func (m *memFS) MkdirTemp(string, string) (string, error) { return "", errors.New("unused") }
func (m *memFS) Rename(string, string) error              { return errors.New("unused") }

const py = ".venv/bin/python"

var venv = map[string]string{".venv/pyvenv.cfg": "", ".venv/bin/python": ""}

func with(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func newPIP(t *testing.T, m *memFS) *PIP {
	t.Helper()
	p, err := newFor(m, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, m *memFS, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	req.Dir = proj
	return newPIP(t, m).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func respond(m map[string]string) *execx.FakeRunner {
	f := &execx.FakeRunner{Responses: map[string]execx.Response{}}
	for cmd, out := range m {
		f.Responses[cmd] = execx.Response{Output: execx.Output{Stdout: out}}
	}
	return f
}

func TestContractSuite(t *testing.T) {
	contract.RunPackageManagerSuite(t, newPIP(t, newMem(nil)))
}

func TestCapabilityTable(t *testing.T) {
	caps := newPIP(t, newMem(nil)).Capabilities()
	want := map[core.Capability]core.Level{
		core.CapInit: core.LevelFilled, core.CapInstall: core.LevelNative, core.CapAdd: core.LevelFilled,
		core.CapRemove: core.LevelFilled, core.CapList: core.LevelNative, core.CapUpdate: core.LevelFilled,
		core.CapOutdated: core.LevelNative, core.CapRun: core.LevelFilled, core.CapBuild: core.LevelFilled,
		core.CapTest: core.LevelFilled, core.CapLint: core.LevelFilled, core.CapAudit: core.LevelFilled,
		core.CapPublish: core.LevelFilled, core.CapUp: core.LevelFilled, core.CapDown: core.LevelFilled,
	}
	if len(want) != len(core.AllCapabilities) {
		t.Fatalf("table covers %d of %d capabilities", len(want), len(core.AllCapabilities))
	}
	for c, level := range want {
		if caps[c].Level != level {
			t.Errorf("%s = %q, want %q", c, caps[c].Level, level)
		}
	}
}

func TestArgvPerCapability(t *testing.T) {
	reqs := map[string]string{"requirements.txt": "rich==13.0\n"}
	cases := []struct {
		name  string
		cap   core.Capability
		files map[string]string
		req   core.Request
		want  []string
	}{
		{"install creates venv", core.CapInstall, reqs, core.Request{},
			[]string{"python3 -m venv .venv", py + " -m pip install -r requirements.txt"}},
		{"install in existing venv", core.CapInstall, with(reqs, venv), core.Request{Args: []string{"--no-cache-dir"}},
			[]string{py + " -m pip install -r requirements.txt --no-cache-dir"}},
		{"install dev group", core.CapInstall, with(venv, map[string]string{"requirements-dev.txt": ""}), core.Request{Group: "dev"},
			[]string{py + " -m pip install -r requirements-dev.txt"}},
		{"install env groups", core.CapInstall, with(venv, map[string]string{"requirements-dev.txt": "", "requirements-docs.txt": ""}), core.Request{Groups: []string{"dev", "docs"}},
			[]string{py + " -m pip install -r requirements-dev.txt -r requirements-docs.txt"}},
		{"add", core.CapAdd, venv, core.Request{Packages: []string{"rich", "attrs>=23"}},
			[]string{py + " -m pip install rich attrs>=23", py + " -m pip freeze"}},
		{"add creates venv", core.CapAdd, nil, core.Request{Packages: []string{"rich"}},
			[]string{"python3 -m venv .venv", py + " -m pip install rich", py + " -m pip freeze"}},
		{"remove", core.CapRemove, with(venv, reqs), core.Request{Packages: []string{"rich"}},
			[]string{py + " -m pip uninstall --yes rich"}},
		{"update named", core.CapUpdate, with(venv, reqs), core.Request{Packages: []string{"rich"}},
			[]string{py + " -m pip install --upgrade rich", py + " -m pip freeze"}},
		{"update all", core.CapUpdate, with(venv, reqs), core.Request{},
			[]string{py + " -m pip install --upgrade -r requirements.txt", py + " -m pip freeze"}},
		{"list", core.CapList, venv, core.Request{}, []string{py + " -m pip list --format json"}},
		{"outdated", core.CapOutdated, venv, core.Request{}, []string{py + " -m pip list --format json --outdated"}},
		{"build", core.CapBuild, venv, core.Request{Args: []string{"--wheel"}},
			[]string{py + ` -c import importlib.util,sys; sys.exit(0 if importlib.util.find_spec("build") else 1)`, py + " -m build --wheel"}},
		{"audit", core.CapAudit, venv, core.Request{},
			[]string{py + ` -c import importlib.util,sys; sys.exit(0 if importlib.util.find_spec("pip_audit") else 1)`, py + " -m pip_audit"}},
		{"publish", core.CapPublish, venv, core.Request{},
			[]string{py + ` -c import importlib.util,sys; sys.exit(0 if importlib.util.find_spec("twine") else 1)`, py + " -m twine upload dist/*"}},
		{"init", core.CapInit, nil, core.Request{}, []string{"python3 -m venv .venv"}},
		{"init keeps venv", core.CapInit, venv, core.Request{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, newMem(c.files), c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); len(got)+len(c.want) > 0 && !reflect.DeepEqual(got, c.want) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTestAndLintFills(t *testing.T) {
	bin := with(venv, map[string]string{".venv/bin/pytest": "", ".venv/bin/ruff": ""})
	cases := []struct {
		name  string
		cap   core.Capability
		files map[string]string
		req   core.Request
		want  string
	}{
		{"pytest from requirements", core.CapTest, with(bin, map[string]string{"requirements-dev.txt": "pytest==8\n"}), core.Request{Args: []string{"-x"}}, ".venv/bin/pytest -x"},
		{"pytest from pyproject dep", core.CapTest, map[string]string{"pyproject.toml": "[project]\ndependencies = [\"pytest\"]\n"}, core.Request{}, "pytest"},
		{"pytest config file", core.CapTest, with(bin, map[string]string{"pytest.ini": "[pytest]\n"}), core.Request{}, ".venv/bin/pytest"},
		{"test script", core.CapTest, with(bin, map[string]string{"pyproject.toml": "[tool.upc.scripts]\ntest = \"tox -e py\"\n"}), core.Request{}, "tox -e py"},
		{"ruff", core.CapLint, with(bin, map[string]string{"ruff.toml": ""}), core.Request{}, ".venv/bin/ruff check"},
		{"lint script", core.CapLint, with(bin, map[string]string{"pyproject.toml": "[tool.upc.scripts]\nlint = \"ruff check src\"\n"}), core.Request{}, ".venv/bin/ruff check src"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, newMem(c.files), c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) || !fake.Calls[0].Stream {
				t.Errorf("argv = %q (stream %v), want %q", got, fake.Calls[0].Stream, c.want)
			}
		})
	}
}

func TestUndeclaredToolIsUnsupportedWithHowToEnable(t *testing.T) {
	for c, hint := range map[core.Capability]string{core.CapTest: "upc add --dev pytest", core.CapLint: "upc add --dev ruff"} {
		fake := &execx.FakeRunner{}
		_, err := call(t, newMem(venv), c, core.Request{}, fake)
		if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), hint) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %v", c, err, argv(fake))
		}
		var ue *core.UnsupportedError
		if !errors.As(err, &ue) || ue.PackageManager != ID || ue.Capability != c {
			t.Errorf("%s: %+v", c, ue)
		}
	}
}

func TestRunLookupOrder(t *testing.T) {
	files := with(venv, map[string]string{
		"pyproject.toml":   "[project.scripts]\nserve = \"demo:main\"\nboth = \"demo:one\"\n[tool.upc.scripts]\ncheck = \"pytest -q\"\nboth = \"echo upc\"\n",
		"serve.py":         "",
		".venv/bin/pytest": "",
		".venv/bin/serve":  "",
	})
	cases := []struct{ name, script, want string }{
		{"file", "serve.py", py + " serve.py a"},
		{"command", "check", ".venv/bin/pytest -q a"},
		{"command beats entry point", "both", "echo upc a"},
		{"entry point", "serve", ".venv/bin/serve a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, newMem(files), core.CapRun, core.Request{Script: c.script, Args: []string{"a"}}, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) || !fake.Calls[0].Stream {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
		})
	}
	for _, bad := range []string{"", "--help", "missing"} {
		fake := &execx.FakeRunner{}
		if _, err := call(t, newMem(files), core.CapRun, core.Request{Script: bad}, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("script %q: err = %v", bad, err)
		}
	}
}

func TestMissingModuleOffersInstall(t *testing.T) {
	check := py + ` -c import importlib.util,sys; sys.exit(0 if importlib.util.find_spec("build") else 1)`
	cmd := execx.Cmd{Name: py, Args: []string{"-c", `import importlib.util,sys; sys.exit(0 if importlib.util.find_spec("build") else 1)`}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{check: {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	_, err := call(t, newMem(venv), core.CapBuild, core.Request{}, fake)
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) || missing.Tool != "build" || !reflect.DeepEqual(missing.InstallCmd, []string{py, "-m", "pip", "install", "build"}) {
		t.Fatalf("err = %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Errorf("must not run build after a failed check: %q", argv(fake))
	}
}

func TestActingWithoutVenvIsRefused(t *testing.T) {
	for _, c := range []core.Capability{core.CapList, core.CapOutdated, core.CapRemove, core.CapBuild, core.CapAudit, core.CapPublish} {
		fake := &execx.FakeRunner{}
		_, err := call(t, newMem(map[string]string{"requirements.txt": ""}), c, core.Request{Packages: []string{"x"}}, fake)
		if !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %v", c, err, argv(fake))
		}
	}
}

func TestInitCreatesEmptyRequirementsOnce(t *testing.T) {
	m := newMem(nil)
	if _, err := call(t, m, core.CapInit, core.Request{}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if body, ok := m.files["/proj/requirements.txt"]; !ok || body != "" {
		t.Fatalf("requirements.txt = %q, %v", body, ok)
	}
	m.files["/proj/requirements.txt"] = "rich==1\n"
	if _, err := call(t, m, core.CapInit, core.Request{}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if m.files["/proj/requirements.txt"] != "rich==1\n" {
		t.Error("init must not overwrite an existing requirements.txt")
	}
}

func TestInitScript(t *testing.T) {
	m := newMem(nil)
	fake := &execx.FakeRunner{}
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "script", Script: "hello"}, fake); err != nil {
		t.Fatal(err)
	}
	if body := m.files["/proj/hello.py"]; !strings.HasPrefix(body, "# /// script\n") || len(fake.Calls) != 0 {
		t.Errorf("hello.py = %q, ran %q", body, argv(fake))
	}
	for name, req := range map[string]core.Request{
		"exists":       {ProjectType: "script", Script: "hello"},
		"bad name":     {ProjectType: "script", Script: "--x"},
		"path in name": {ProjectType: "script", Script: "a/b"},
		"other type":   {ProjectType: "spaceship"},
	} {
		if _, err := call(t, m, core.CapInit, req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestInitPackage(t *testing.T) {
	m := newMem(nil)
	fake := &execx.FakeRunner{}
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "My-Lib"}, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("ran %q; pip package init only writes files", argv(fake))
	}
	for _, f := range []string{"pyproject.toml", "README.md", "requirements.txt", "src/my_lib/__init__.py"} {
		if _, ok := m.files[filepath.Join(proj, "My-Lib", f)]; !ok {
			t.Errorf("missing %s; have %v", f, m.files)
		}
	}
	if body := m.files[filepath.Join(proj, "My-Lib", "pyproject.toml")]; !strings.Contains(body, `name = "My-Lib"`) {
		t.Errorf("pyproject.toml = %q", body)
	}
}

func TestInitPackageDryRunWritesNothing(t *testing.T) {
	m := newMem(nil)
	res, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "demo", DryRun: true}, &execx.FakeRunner{})
	if err != nil || len(m.files) != 0 || !strings.HasPrefix(res.Summary, "dry run") {
		t.Errorf("res = %+v, err = %v, files = %v", res, err, m.files)
	}
}

func TestInitPackageRejectsBadInput(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		req   core.Request
	}{
		"no name":      {nil, core.Request{ProjectType: "package"}},
		"flag-like":    {nil, core.Request{ProjectType: "package", Name: "--x"}},
		"path in name": {nil, core.Request{ProjectType: "package", Name: "a/b"}},
		"exists":       {map[string]string{"demo": ""}, core.Request{ProjectType: "package", Name: "demo"}},
	} {
		if _, err := call(t, newMem(tc.files), core.CapInit, tc.req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestInitPackageFailureLeavesNothing(t *testing.T) {
	m := newMem(nil)
	m.failWrite = errors.New("disk full")
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "demo"}, &execx.FakeRunner{}); err == nil {
		t.Fatal("want an error")
	}
	if len(m.files) != 0 {
		t.Errorf("files = %v", m.files)
	}
}

func TestListAndOutdatedParse(t *testing.T) {
	jsonOut := `[{"name":"rich","version":"13.0","latest_version":"14.0"},{"name":"attrs","version":"23.1","latest_version":"23.1"}]`
	res, err := call(t, newMem(venv), core.CapList, core.Request{}, respond(map[string]string{py + " -m pip list --format json": jsonOut}))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 2 || res.Summary != "rich 13.0\nattrs 23.1" {
		t.Errorf("list = %+v", res)
	}
	res, err = call(t, newMem(venv), core.CapOutdated, core.Request{}, respond(map[string]string{py + " -m pip list --format json --outdated": jsonOut}))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 1 || got[0].Name != "rich" || res.Summary != "rich 13.0 -> 14.0" {
		t.Errorf("outdated = %+v", res)
	}
	if _, err := call(t, newMem(venv), core.CapList, core.Request{}, respond(map[string]string{py + " -m pip list --format json": "nope"})); err == nil {
		t.Error("garbled output must be an error")
	}
	res, err = call(t, newMem(venv), core.CapList, core.Request{}, &execx.FakeRunner{})
	if err != nil || res.Summary != "no dependencies" {
		t.Errorf("empty = %+v, %v", res, err)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	m := newMem(with(venv, map[string]string{"requirements.txt": "rich==1\n"}))
	fake := respond(map[string]string{py + " -m pip freeze": "attrs==23.1\n"})
	for _, c := range []core.Capability{core.CapAdd, core.CapRemove, core.CapUpdate} {
		res, err := call(t, m, c, core.Request{Packages: []string{"attrs"}, DryRun: true}, fake)
		if err != nil || !strings.HasPrefix(res.Summary, "dry run, nothing changed: would ") {
			t.Errorf("%s: %+v, %v", c, res, err)
		}
	}
	if m.writes != 0 || m.files["/proj/requirements.txt"] != "rich==1\n" {
		t.Errorf("dry run wrote: writes=%d file=%q", m.writes, m.files["/proj/requirements.txt"])
	}
}

func TestDetectAndRank(t *testing.T) {
	p := newPIP(t, newMem(nil))
	if !p.Detect(fstest.MapFS{"requirements.txt": {}}) || p.DetectRank(fstest.MapFS{"requirements.txt": {}}) != rankRequirements {
		t.Error("requirements.txt must be detected at the requirements rank")
	}
	if p.Detect(fstest.MapFS{"pyproject.toml": {}}) || p.DetectRank(fstest.MapFS{"pyproject.toml": {}}) != 0 {
		t.Error("pyproject.toml alone is not pip")
	}
}

func TestDiagnoseMissingVenv(t *testing.T) {
	p := newPIP(t, newMem(nil))
	got := p.Diagnose(fstest.MapFS{"requirements.txt": {}})
	if len(got) != 1 || got[0].Fix != core.CapInstall {
		t.Errorf("findings = %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{"requirements.txt": {}, ".venv/pyvenv.cfg": {}}); len(got) != 0 {
		t.Errorf("with venv: %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{}); len(got) != 0 {
		t.Errorf("empty: %+v", got)
	}
}

func TestToolInstallInfo(t *testing.T) {
	for goos, wantCmd := range map[string]string{"darwin": "brew install python", "linux": "", "windows": ""} {
		tl := tool(goos)
		if got := strings.Join(tl.InstallCmd, " "); got != wantCmd || tl.InstallHint == "" || tl.Binary == "" {
			t.Errorf("%s: %+v", goos, tl)
		}
	}
	if tool("windows").Binary != "python" || tool("linux").Binary != "python3" {
		t.Error("interpreter name per platform")
	}
}
