package pip

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

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
