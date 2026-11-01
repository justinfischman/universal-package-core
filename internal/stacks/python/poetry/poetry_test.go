package poetry

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

const pyproject = "[tool.poetry]\nname = \"demo\"\nversion = \"0.1.0\"\n"

var projectFiles = map[string]string{"pyproject.toml": pyproject}

// fixture creates a project directory with the given files.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newPoetry(t *testing.T) *Poetry {
	t.Helper()
	p, err := newFor("linux")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	return newPoetry(t).Execute(execx.WithRunner(t.Context(), fake), c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func TestContractSuite(t *testing.T) {
	contract.RunPackageManagerSuite(t, newPoetry(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newPoetry(t).Capabilities()
	want := map[core.Capability]core.Level{
		core.CapInit: core.LevelNative, core.CapInstall: core.LevelNative, core.CapAdd: core.LevelNative,
		core.CapRemove: core.LevelNative, core.CapList: core.LevelNative, core.CapUpdate: core.LevelNative,
		core.CapOutdated: core.LevelNative, core.CapRun: core.LevelNative, core.CapBuild: core.LevelNative,
		core.CapPublish: core.LevelNative,
		core.CapTest:    core.LevelFilled, core.CapLint: core.LevelFilled, core.CapAudit: core.LevelFilled,
		core.CapUp: core.LevelUnsupported, core.CapDown: core.LevelUnsupported,
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
		name string
		cap  core.Capability
		req  core.Request
		want string
	}{
		{"init", core.CapInit, core.Request{}, "poetry init --no-interaction"},
		{"install", core.CapInstall, core.Request{}, "poetry install"},
		{"install group", core.CapInstall, core.Request{Group: "docs"}, "poetry install --with docs"},
		{"add", core.CapAdd, core.Request{Packages: []string{"requests", "rich>=13"}}, "poetry add requests rich>=13"},
		{"add dev", core.CapAdd, core.Request{Packages: []string{"pytest"}, Group: "dev"}, "poetry add --group dev pytest"},
		{"add passthrough", core.CapAdd, core.Request{Packages: []string{"rich"}, Args: []string{"--lock"}}, "poetry add rich --lock"},
		{"remove", core.CapRemove, core.Request{Packages: []string{"rich"}, Group: "docs"}, "poetry remove --group docs rich"},
		{"list", core.CapList, core.Request{}, "poetry show --top-level"},
		{"outdated", core.CapOutdated, core.Request{}, "poetry show --top-level --outdated"},
		{"update all", core.CapUpdate, core.Request{}, "poetry update"},
		{"update named", core.CapUpdate, core.Request{Packages: []string{"a", "b"}}, "poetry update a b"},
		{"build", core.CapBuild, core.Request{Args: []string{"--format", "wheel"}}, "poetry build --format wheel"},
		{"publish", core.CapPublish, core.Request{}, "poetry publish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, projectFiles)
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if fake.Calls[0].Dir != c.req.Dir {
				t.Errorf("dir = %q, want %q", fake.Calls[0].Dir, c.req.Dir)
			}
		})
	}
}

func TestDryRunStillRoutesButSummaryPlansOnly(t *testing.T) {
	for _, c := range []core.Capability{core.CapInstall, core.CapList, core.CapOutdated, core.CapUpdate, core.CapBuild} {
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

func TestInitPackage(t *testing.T) {
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "package", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	if got := argv(fake); !reflect.DeepEqual(got, []string{"poetry new --src demo"}) {
		t.Errorf("ran %q", got)
	}
	for name, req := range map[string]core.Request{
		"no name":   {ProjectType: "package"},
		"flag-like": {ProjectType: "package", Name: "--x"},
		"path":      {ProjectType: "package", Name: "a/b"},
	} {
		req.Dir = t.TempDir()
		fake := &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %q", name, err, argv(fake))
		}
	}
}

func TestRejectsBadInput(t *testing.T) {
	dir := fixture(t, projectFiles)
	cases := map[string]struct {
		cap core.Capability
		req core.Request
	}{
		"add no packages":   {core.CapAdd, core.Request{Dir: dir}},
		"add flag-like pkg": {core.CapAdd, core.Request{Dir: dir, Packages: []string{"--source=evil"}}},
		"add flag group":    {core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "--bad"}},
		"add script":        {core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Script: "a.py"}},
		"add empty dir":     {core.CapAdd, core.Request{Dir: t.TempDir(), Packages: []string{"x"}}},
		"install flag":      {core.CapInstall, core.Request{Dir: dir, Group: "-x"}},
		"update flag-like":  {core.CapUpdate, core.Request{Dir: dir, Packages: []string{"-x"}}},
		"init script":       {core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "a"}},
		"init unknown":      {core.CapInit, core.Request{Dir: dir, ProjectType: "spaceship"}},
		"list empty dir":    {core.CapList, core.Request{Dir: t.TempDir()}},
		"test empty dir":    {core.CapTest, core.Request{Dir: t.TempDir()}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
				t.Errorf("err = %v, ran %v", err, argv(fake))
			}
		})
	}
}

func TestListAndOutdatedParseOutput(t *testing.T) {
	dir := fixture(t, projectFiles)
	respond := func(args, out string) *execx.FakeRunner {
		return &execx.FakeRunner{Responses: map[string]execx.Response{args: {Output: execx.Output{Stdout: out}}}}
	}

	res, err := call(t, core.CapList, core.Request{Dir: dir},
		respond("poetry show --top-level", "requests 2.31.0 Python HTTP for Humans.\nrich    15.0.0  Render rich text\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Name: "requests", Version: "2.31.0"}, {Name: "rich", Version: "15.0.0"}}
	if !reflect.DeepEqual(res.Data, want) || res.Summary != "requests 2.31.0\nrich 15.0.0" {
		t.Errorf("list = %+v", res)
	}

	res, err = call(t, core.CapOutdated, core.Request{Dir: dir},
		respond("poetry show --top-level --outdated", "requests 2.31.0 2.34.2 Python HTTP\nrich (!) 15.0.0 15.0.0 text\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Data, []core.Package{{Name: "requests", Version: "2.31.0", Latest: "2.34.2"}}) ||
		res.Summary != "requests 2.31.0 -> 2.34.2" {
		t.Errorf("outdated = %+v", res)
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

func TestNonZeroExitIsReturnedAsExitError(t *testing.T) {
	cmd := execx.Cmd{Name: "poetry", Args: []string{"add", "nope"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1, Stderr: "package not found"}},
	}}
	_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"nope"}}, fake)
	var exit *execx.ExitError
	if !errors.As(err, &exit) || exit.Code != 1 || core.ExitCode(err) != core.ExitFailure {
		t.Fatalf("err = %v, want ExitError code 1", err)
	}
}

func TestRunLookupOrder(t *testing.T) {
	files := map[string]string{
		"pyproject.toml": pyproject + `[tool.poetry.scripts]
serve = "demo:main"
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
		{"file beats entries", core.Request{Script: "serve.py", Args: []string{"x"}}, "poetry run python serve.py x"},
		{"upc script", core.Request{Script: "test", Args: []string{"-k", "a"}}, "poetry run pytest -q my tests -k a"},
		{"upc beats project", core.Request{Script: "both"}, "poetry run echo upc"},
		{"project script", core.Request{Script: "serve", Args: []string{"--port", "1"}}, "poetry run serve --port 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, files)
			fake := &execx.FakeRunner{}
			if _, err := call(t, core.CapRun, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) || !fake.Calls[0].Stream {
				t.Errorf("argv = %q (stream %v), want %q", got, fake.Calls[0].Stream, c.want)
			}
		})
	}
}

func TestRunRejectsBadNames(t *testing.T) {
	dir := fixture(t, map[string]string{"pyproject.toml": pyproject + "[tool.upc.scripts]\nok = 'echo hi'\n"})
	for _, name := range []string{"", "--help", "missing"} {
		fake := &execx.FakeRunner{}
		_, err := call(t, core.CapRun, core.Request{Dir: dir, Script: name}, fake)
		if !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("name %q: err = %v, ran %v", name, err, argv(fake))
		}
	}
}

func TestTestLintAuditAreFilled(t *testing.T) {
	declared := func(extra string) map[string]string {
		return map[string]string{"pyproject.toml": pyproject + extra}
	}
	cases := []struct {
		name  string
		cap   core.Capability
		files map[string]string
		req   core.Request
		want  string
	}{
		{"pytest from poetry group", core.CapTest, declared("[tool.poetry.group.dev.dependencies]\npytest = \"^8\"\n"), core.Request{Args: []string{"-k", "a"}}, "poetry run pytest -k a"},
		{"pytest from tool table", core.CapTest, declared("[tool.pytest.ini_options]\naddopts = \"-q\"\n"), core.Request{}, "poetry run pytest"},
		{"test script wins", core.CapTest, declared("[tool.upc.scripts]\ntest = \"tox -e py\"\n"), core.Request{}, "poetry run tox -e py"},
		{"ruff from dependencies", core.CapLint, declared("[tool.poetry.dependencies]\npython = \"^3.11\"\nruff = \"*\"\n"), core.Request{}, "poetry run ruff check"},
		{"pip-audit declared", core.CapAudit, declared("[tool.poetry.group.dev.dependencies]\npip-audit = \"*\"\n"), core.Request{Args: []string{"--strict"}}, "poetry run pip-audit --strict"},
		{"audit script wins", core.CapAudit, declared("[tool.upc.scripts]\naudit = \"safety check\"\n"), core.Request{}, "poetry run safety check"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, c.files)
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) || !fake.Calls[0].Stream {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
		})
	}
}

func TestUndeclaredToolIsUnsupportedWithHowToEnable(t *testing.T) {
	for c, hint := range map[core.Capability]string{
		core.CapTest: "upc add --dev pytest", core.CapLint: "upc add --dev ruff", core.CapAudit: "upc add --dev pip-audit",
	} {
		fake := &execx.FakeRunner{}
		_, err := call(t, c, core.Request{Dir: fixture(t, projectFiles)}, fake)
		var ue *core.UnsupportedError
		if !errors.As(err, &ue) || ue.PackageManager != ID || !strings.Contains(err.Error(), hint) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %v", c, err, argv(fake))
		}
	}
}

func TestUnsupportedCapabilitiesNamePoetryAndReason(t *testing.T) {
	for _, c := range []core.Capability{core.CapUp, core.CapDown} {
		_, err := newPoetry(t).Execute(t.Context(), c, core.Request{})
		var ue *core.UnsupportedError
		if !errors.As(err, &ue) || ue.PackageManager != ID || ue.Reason == "" || core.ExitCode(err) != core.ExitUnsupported {
			t.Errorf("%s: err = %v", c, err)
		}
	}
}

func TestMissingToolCarriesInstallInfo(t *testing.T) {
	for goos, wantCmd := range map[string]string{"darwin": "brew install poetry", "linux": "", "windows": ""} {
		p, err := newFor(goos)
		if err != nil {
			t.Fatal(err)
		}
		fake := &execx.FakeRunner{Responses: map[string]execx.Response{
			"poetry add rich": {Err: &core.ToolMissingError{Tool: "poetry"}},
		}}
		_, err = p.Execute(execx.WithRunner(t.Context(), fake), core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"rich"}})
		var missing *core.ToolMissingError
		if !errors.As(err, &missing) || missing.InstallHint == "" || core.ExitCode(err) != core.ExitToolMissing {
			t.Fatalf("%s: err = %v", goos, err)
		}
		if got := strings.Join(missing.InstallCmd, " "); got != wantCmd {
			t.Errorf("%s: install cmd = %q, want %q", goos, got, wantCmd)
		}
	}
}

func TestRunnerRequiredInContext(t *testing.T) {
	_, err := newPoetry(t).Execute(t.Context(), core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "runner") {
		t.Errorf("err = %v", err)
	}
}

func TestDetectAndRank(t *testing.T) {
	cases := []struct {
		name string
		fsys fstest.MapFS
		want int
	}{
		{"lockfile", fstest.MapFS{"poetry.lock": {}, "requirements.txt": {}}, rankLockfile},
		{"lockfile beats config", fstest.MapFS{"poetry.lock": {}, "pyproject.toml": {Data: []byte(pyproject)}}, rankLockfile},
		{"tool.poetry", fstest.MapFS{"pyproject.toml": {Data: []byte(pyproject)}}, rankConfig},
		{"plain pyproject", fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\nname = \"d\"\n")}}, 0},
		{"broken pyproject", fstest.MapFS{"pyproject.toml": {Data: []byte("[[[")}}, 0},
		{"tool.uv only", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n")}}, 0},
		{"requirements only", fstest.MapFS{"requirements.txt": {}}, 0},
		{"empty", fstest.MapFS{}, 0},
	}
	p := newPoetry(t)
	for _, c := range cases {
		if got := p.DetectRank(c.fsys); got != c.want {
			t.Errorf("%s: rank = %d, want %d", c.name, got, c.want)
		}
		if got := p.Detect(c.fsys); got != (c.want > 0) {
			t.Errorf("%s: Detect = %v", c.name, got)
		}
	}
}

func TestDiagnose(t *testing.T) {
	p := newPoetry(t)
	unlocked := fstest.MapFS{"pyproject.toml": {Data: []byte(pyproject)}}
	got := p.Diagnose(unlocked)
	if len(got) != 1 || got[0].Fix != core.CapInstall || got[0].Severity != core.SeverityWarn {
		t.Errorf("missing lock: %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{"pyproject.toml": unlocked["pyproject.toml"], "poetry.lock": {}}); len(got) != 0 {
		t.Errorf("locked project: %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{"requirements.txt": {}}); len(got) != 0 {
		t.Errorf("not a poetry project: %+v", got)
	}
}
