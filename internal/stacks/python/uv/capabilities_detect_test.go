package uv

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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
