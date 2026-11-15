package uv

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

func TestContractSuite(t *testing.T) {
	contract.RunPackageManagerSuite(t, newUV(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newUV(t).Capabilities()
	want := map[core.Capability]core.Level{
		core.CapInit: core.LevelNative, core.CapInstall: core.LevelNative, core.CapAdd: core.LevelNative,
		core.CapRemove: core.LevelNative, core.CapList: core.LevelNative, core.CapUpdate: core.LevelNative,
		core.CapOutdated: core.LevelNative, core.CapAudit: core.LevelNative, core.CapRun: core.LevelNative,
		core.CapBuild: core.LevelNative, core.CapPublish: core.LevelNative,
		core.CapTest: core.LevelFilled, core.CapLint: core.LevelFilled,
		core.CapUp: core.LevelFilled, core.CapDown: core.LevelFilled,
	}
	if len(want) != len(core.AllCapabilities) {
		t.Fatalf("test table covers %d of %d capabilities", len(want), len(core.AllCapabilities))
	}
	for c, level := range want {
		if caps[c].Level != level {
			t.Errorf("%s = %q, want %q", c, caps[c].Level, level)
		}
	}
}

func TestArgvPerCapability(t *testing.T) {
	cases := []struct {
		name  string
		cap   core.Capability
		files map[string]string
		req   core.Request
		want  []string
	}{
		{"init script", core.CapInit, nil, core.Request{ProjectType: "script", Script: "hello"}, []string{"uv init --script hello.py"}},
		{"init script keeps extension", core.CapInit, nil, core.Request{ProjectType: "script", Script: "tool.py"}, []string{"uv init --script tool.py"}},
		{"init package", core.CapInit, nil, core.Request{ProjectType: "package", Name: "demo"}, []string{"uv init --package --vcs none --no-workspace demo"}},
		{"init app", core.CapInit, map[string]string{"demo/pyproject.toml": "[project]\nname = \"demo\"\n"}, core.Request{ProjectType: "app", Name: "demo"}, []string{"uv init --app --vcs none --no-workspace demo"}},
		{"init plain", core.CapInit, nil, core.Request{}, []string{"uv init"}},
		{"install project", core.CapInstall, projectFiles, core.Request{}, []string{"uv sync"}},
		{"install group", core.CapInstall, projectFiles, core.Request{Group: "docs"}, []string{"uv sync --group docs"}},
		{"install script", core.CapInstall, scriptFiles, core.Request{}, []string{"uv sync --script hello.py"}},
		{"install requirements", core.CapInstall, map[string]string{"requirements.txt": "rich\n"}, core.Request{},
			[]string{"uv venv", "uv pip install -r requirements.txt"}},
		{"install requirements with venv", core.CapInstall, map[string]string{"requirements.txt": "rich\n", ".venv/pyvenv.cfg": ""}, core.Request{},
			[]string{"uv pip install -r requirements.txt"}},
		{"remove", core.CapRemove, projectFiles, core.Request{Packages: []string{"rich"}, Group: "dev"}, []string{"uv remove --dev rich"}},
		{"remove script", core.CapRemove, scriptFiles, core.Request{Packages: []string{"rich"}}, []string{"uv remove --script hello.py rich"}},
		{"list project", core.CapList, projectFiles, core.Request{}, []string{"uv tree --depth 1"}},
		{"list script", core.CapList, scriptFiles, core.Request{}, []string{"uv tree --script hello.py --depth 0"}},
		{"list requirements", core.CapList, map[string]string{"requirements.txt": ""}, core.Request{}, []string{"uv pip list --format json"}},
		{"outdated project", core.CapOutdated, projectFiles, core.Request{}, []string{"uv tree --depth 1 --outdated"}},
		{"outdated script", core.CapOutdated, scriptFiles, core.Request{}, []string{"uv tree --script hello.py --depth 0 --outdated"}},
		{"outdated requirements", core.CapOutdated, map[string]string{"requirements.txt": ""}, core.Request{}, []string{"uv pip list --format json --outdated"}},
		{"update all", core.CapUpdate, projectFiles, core.Request{}, []string{"uv lock --upgrade", "uv sync"}},
		{"update named", core.CapUpdate, projectFiles, core.Request{Packages: []string{"a", "b"}},
			[]string{"uv lock --upgrade-package a --upgrade-package b", "uv sync"}},
		{"update script", core.CapUpdate, scriptFiles, core.Request{}, []string{"uv lock --script hello.py --upgrade", "uv sync --script hello.py"}},
		{"update requirements", core.CapUpdate, map[string]string{"requirements.txt": ""}, core.Request{}, []string{"uv pip install --upgrade -r requirements.txt"}},
		{"audit project", core.CapAudit, projectFiles, core.Request{}, []string{"uv audit"}},
		{"audit script", core.CapAudit, scriptFiles, core.Request{}, []string{"uv audit --script hello.py"}},
		{"build", core.CapBuild, projectFiles, core.Request{Args: []string{"--wheel"}}, []string{"uv build --wheel"}},
		{"publish", core.CapPublish, projectFiles, core.Request{}, []string{"uv publish"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, c.files)
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, c.want) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDryRunStillRoutesButSummaryPlansOnly(t *testing.T) {
	for _, c := range []core.Capability{core.CapInstall, core.CapList, core.CapOutdated, core.CapUpdate, core.CapAudit} {
		t.Run(string(c), func(t *testing.T) {
			res, err := call(t, c, core.Request{Dir: fixture(t, projectFiles), DryRun: true}, &execx.FakeRunner{})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(res.Summary, "dry run, nothing changed: would ") || len(res.Commands) == 0 {
				t.Errorf("result = %+v", res)
			}
		})
	}
}

func TestListAndOutdatedParseOutput(t *testing.T) {
	tree := "demo v0.1.0\n├── requests v2.31.0 (latest: v2.34.2)\n└── rich v15.0.0\n"
	dir := fixture(t, projectFiles)
	respond := func(args string) *execx.FakeRunner {
		return &execx.FakeRunner{Responses: map[string]execx.Response{args: {Output: execx.Output{Stdout: tree}}}}
	}

	res, err := call(t, core.CapList, core.Request{Dir: dir}, respond("uv tree --depth 1"))
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Name: "requests", Version: "2.31.0", Latest: "2.34.2"}, {Name: "rich", Version: "15.0.0"}}
	if !reflect.DeepEqual(res.Data, want) || res.Summary != "requests 2.31.0\nrich 15.0.0" {
		t.Errorf("list = %+v", res)
	}

	res, err = call(t, core.CapOutdated, core.Request{Dir: dir}, respond("uv tree --depth 1 --outdated"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Data, want[:1]) || res.Summary != "requests 2.31.0 -> 2.34.2" {
		t.Errorf("outdated = %+v", res)
	}
}

func TestListScriptAndRequirementsParsing(t *testing.T) {
	res, err := call(t, core.CapList, core.Request{Dir: fixture(t, scriptFiles)},
		&execx.FakeRunner{Responses: map[string]execx.Response{
			"uv tree --script hello.py --depth 0": {Output: execx.Output{Stdout: "rich v15.0.0\nrequests v2.31.0\n"}},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 2 || got[0].Name != "rich" {
		t.Errorf("script list = %+v", got)
	}

	jsonOut := `[{"name":"rich","version":"1.0","latest_version":"2.0"}]`
	res, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, map[string]string{"requirements.txt": ""})},
		&execx.FakeRunner{Responses: map[string]execx.Response{
			"uv pip list --format json --outdated": {Output: execx.Output{Stdout: jsonOut}},
		}})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 1 || got[0].Latest != "2.0" {
		t.Errorf("requirements outdated = %+v", got)
	}

	_, err = call(t, core.CapList, core.Request{Dir: fixture(t, map[string]string{"requirements.txt": ""})},
		&execx.FakeRunner{Responses: map[string]execx.Response{
			"uv pip list --format json": {Output: execx.Output{Stdout: "not json"}},
		}})
	if err == nil {
		t.Error("garbled package list must be an error")
	}
}

func TestListEmptyIsReported(t *testing.T) {
	res, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, &execx.FakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "no dependencies" || len(res.Data.([]core.Package)) != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestListSurfacesStderrOfFailedCommand(t *testing.T) {
	cmd := execx.Cmd{Name: "uv", Args: []string{"tree", "--depth", "1"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 2, Stderr: "lockfile out of date"}},
	}}
	_, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, fake)
	if err == nil || !strings.Contains(err.Error(), "lockfile out of date") {
		t.Errorf("err = %v, want the tool's stderr", err)
	}
}

func TestRunLookupOrder(t *testing.T) {
	files := map[string]string{
		"pyproject.toml": `[project]
name = "demo"
version = "0.1.0"
[project.scripts]
serve = "demo:main"
both = "demo:one"
[tool.upc.scripts]
test = 'pytest -q "my tests"'
both = "echo upc"
`,
		"serve.py": "print('file')\n",
	}
	cases := []struct {
		name string
		req  core.Request
		want string
	}{
		{"file beats entries", core.Request{Script: "serve.py", Args: []string{"x"}}, "uv run serve.py x"},
		{"upc script", core.Request{Script: "test", Args: []string{"-k", "a"}}, "uv run -- pytest -q my tests -k a"},
		{"upc beats project", core.Request{Script: "both"}, "uv run -- echo upc"},
		{"project script", core.Request{Script: "serve", Args: []string{"--port", "1"}}, "uv run -- serve --port 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, files)
			fake := &execx.FakeRunner{}
			if _, err := call(t, core.CapRun, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if !fake.Calls[0].Stream {
				t.Error("run output must stream to the user")
			}
		})
	}
}

func TestRunKeepsQuotedWordsTogether(t *testing.T) {
	dir := fixture(t, map[string]string{"pyproject.toml": "[tool.upc.scripts]\nq = 'pytest -k \"a b\"'\n"})
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapRun, core.Request{Dir: dir, Script: "q"}, fake); err != nil {
		t.Fatal(err)
	}
	if want := []string{"run", "--", "pytest", "-k", "a b"}; !reflect.DeepEqual(fake.Calls[0].Args, want) {
		t.Errorf("args = %q, want %q", fake.Calls[0].Args, want)
	}
}

func TestRunRejectsBadNames(t *testing.T) {
	dir := fixture(t, map[string]string{"pyproject.toml": "[tool.upc.scripts]\nok = 'echo hi'\n"})
	for _, name := range []string{"", "--help", "missing"} {
		fake := &execx.FakeRunner{}
		_, err := call(t, core.CapRun, core.Request{Dir: dir, Script: name}, fake)
		if !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("name %q: err = %v, ran %v", name, err, argv(fake))
		}
	}
	_, err := call(t, core.CapRun, core.Request{Dir: dir, Script: "missing"}, &execx.FakeRunner{})
	if err == nil || !strings.Contains(err.Error(), "available: ok") {
		t.Errorf("error should list the available scripts: %v", err)
	}
}

func TestRunRejectsBrokenScriptCommand(t *testing.T) {
	dir := fixture(t, map[string]string{"pyproject.toml": "[tool.upc.scripts]\nbad = 'echo \"oops'\n"})
	if _, err := call(t, core.CapRun, core.Request{Dir: dir, Script: "bad"}, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v", err)
	}
}

func TestInitRejectsBadInput(t *testing.T) {
	for name, req := range map[string]core.Request{
		"missing name":    {ProjectType: "script"},
		"flag-like name":  {ProjectType: "script", Script: "--x"},
		"path in name":    {ProjectType: "script", Script: "a/b"},
		"unknown project": {ProjectType: "spaceship", Script: "x"},
		"package no name": {ProjectType: "package"},
		"app no name":     {ProjectType: "app"},
		"app path":        {ProjectType: "app", Name: "a/b"},
		"package flag":    {ProjectType: "package", Name: "--x"},
		"package path":    {ProjectType: "package", Name: "a/b"},
	} {
		fake := &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %v", name, err, argv(fake))
		}
	}
}

func TestMissingToolCarriesInstallInfo(t *testing.T) {
	for goos, wantCmd := range map[string]string{
		"darwin":  "brew install uv",
		"linux":   "sh -c curl -LsSf https://astral.sh/uv/install.sh | sh",
		"windows": "powershell",
	} {
		u, err := newFor(fsx.OS{}, goos)
		if err != nil {
			t.Fatal(err)
		}
		fake := &execx.FakeRunner{Responses: map[string]execx.Response{
			"uv add rich": {Err: &core.ToolMissingError{Tool: "uv"}},
		}}
		ctx := execx.WithRunner(t.Context(), fake)
		_, err = u.Execute(ctx, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"rich"}})
		var missing *core.ToolMissingError
		if !errors.As(err, &missing) {
			t.Fatalf("%s: err = %v", goos, err)
		}
		if missing.InstallHint == "" || len(missing.InstallCmd) == 0 || !strings.HasPrefix(strings.Join(missing.InstallCmd, " "), strings.Fields(wantCmd)[0]) {
			t.Errorf("%s: missing = %+v", goos, missing)
		}
		if core.ExitCode(err) != core.ExitToolMissing {
			t.Errorf("%s: exit code %d", goos, core.ExitCode(err))
		}
	}
	if got := strings.Join(tool("darwin").InstallCmd, " "); got != "brew install uv" {
		t.Errorf("darwin install = %q", got)
	}
}

func TestRunnerRequiredInContext(t *testing.T) {
	_, err := newUV(t).Execute(t.Context(), core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "runner") {
		t.Errorf("err = %v", err)
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"uv.lock", map[string]string{"uv.lock": ""}, true},
		{"tool.uv table", map[string]string{"pyproject.toml": "[tool.uv]\ndev-dependencies = []\n"}, true},
		{"plain pyproject", projectFiles, false},
		{"broken pyproject", map[string]string{"pyproject.toml": "[[["}, false},
		{"standalone script", scriptFiles, true},
		{"requirements only", map[string]string{"requirements.txt": "x"}, false},
		{"empty", nil, false},
	}
	u := newUV(t)
	for _, c := range cases {
		m := fstest.MapFS{}
		for name, body := range c.files {
			m[name] = &fstest.MapFile{Data: []byte(body)}
		}
		if got := u.Detect(m); got != c.want {
			t.Errorf("%s: Detect = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDiagnose(t *testing.T) {
	u := newUV(t)
	pyOnly := fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\nname='x'\n")}}
	got := u.Diagnose(pyOnly)
	if len(got) != 1 || got[0].Fix != core.CapInstall || got[0].Severity != core.SeverityWarn {
		t.Errorf("missing lock: %+v", got)
	}
	locked := fstest.MapFS{"pyproject.toml": pyOnly["pyproject.toml"], "uv.lock": {}}
	if got := u.Diagnose(locked); len(got) != 0 {
		t.Errorf("locked project: %+v", got)
	}
	req := fstest.MapFS{"requirements.txt": {Data: []byte("x")}}
	if got := u.Diagnose(req); len(got) != 1 || got[0].Fix != "" {
		t.Errorf("requirements: %+v", got)
	}
}

func TestTestAndLintAreFilledWithSharedHelpers(t *testing.T) {
	declared := func(extra string) map[string]string {
		return map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n" + extra}
	}
	cases := []struct {
		name  string
		cap   core.Capability
		files map[string]string
		req   core.Request
		want  string
	}{
		{"pytest from dependency", core.CapTest, declared("dependencies = [\"pytest>=8\"]\n"), core.Request{Args: []string{"-k", "a"}}, "uv run -- pytest -k a"},
		{"pytest from tool table", core.CapTest, declared("[tool.pytest.ini_options]\naddopts = \"-q\"\n"), core.Request{}, "uv run -- pytest"},
		{"pytest from requirements", core.CapTest, map[string]string{"pyproject.toml": "[project]\nname = \"d\"\n", "requirements-dev.txt": "pytest==8.0\n"}, core.Request{}, "uv run -- pytest"},
		{"test script wins", core.CapTest, declared("[tool.upc.scripts]\ntest = \"tox -e py\"\n"), core.Request{}, "uv run -- tox -e py"},
		{"ruff from group", core.CapLint, declared("[dependency-groups]\ndev = [\"ruff\"]\n"), core.Request{}, "uv run -- ruff check"},
		{"lint script wins", core.CapLint, declared("[tool.upc.scripts]\nlint = \"flake8 src\"\n"), core.Request{Args: []string{"--x"}}, "uv run -- flake8 src --x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, c.files)
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if !fake.Calls[0].Stream {
				t.Error("output must stream to the user")
			}
		})
	}
}

func TestTestAndLintUndeclaredToolIsUnsupportedWithHowToEnable(t *testing.T) {
	for c, hint := range map[core.Capability]string{core.CapTest: "upc add --dev pytest", core.CapLint: "upc add --dev ruff"} {
		fake := &execx.FakeRunner{}
		_, err := call(t, c, core.Request{Dir: fixture(t, projectFiles)}, fake)
		if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), hint) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %v", c, err, argv(fake))
		}
	}
}

func TestTestNeedsAProject(t *testing.T) {
	_, err := call(t, core.CapTest, core.Request{Dir: fixture(t, map[string]string{"requirements.txt": "pytest\n"})}, &execx.FakeRunner{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v", err)
	}
}

func TestDetectRankOrdersLockfileAboveConfigAboveScript(t *testing.T) {
	u := newUV(t)
	cases := []struct {
		name string
		fsys fstest.MapFS
		want int
	}{
		{"lockfile", fstest.MapFS{"uv.lock": {}, "requirements.txt": {}}, rankLockfile},
		{"tool.uv", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n")}}, rankConfig},
		{"script", fstest.MapFS{"a.py": {Data: []byte(scriptSource)}}, rankScript},
		{"not uv", fstest.MapFS{"requirements.txt": {}}, 0},
	}
	for _, c := range cases {
		if got := u.DetectRank(c.fsys); got != c.want {
			t.Errorf("%s: rank = %d, want %d", c.name, got, c.want)
		}
	}
}
