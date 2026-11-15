package poetry

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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

func TestMissingToolCarriesInstallInfo(t *testing.T) {
	for goos, wantCmd := range map[string]string{"darwin": "brew install poetry", "linux": "", "windows": ""} {
		p, err := newFor(fsx.OS{}, goos)
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
