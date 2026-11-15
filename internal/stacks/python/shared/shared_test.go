package shared

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestToolPlan(t *testing.T) {
	cases := []struct {
		name  string
		tool  Tool
		files fstest.MapFS
		args  []string
		want  []string
	}{
		{"script wins over declaration", Pytest, fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\ndependencies=[\"pytest\"]\n[tool.upc.scripts]\ntest='tox -e \"a b\"'\n")}}, []string{"-x"}, []string{"tox", "-e", "a b", "-x"}},
		{"dependency", Pytest, fstest.MapFS{"pyproject.toml": {Data: []byte("[project]\ndependencies=[\"PyTest>=8\"]\n")}}, nil, []string{"pytest"}},
		{"optional dependency", Pytest, fstest.MapFS{"pyproject.toml": {Data: []byte("[project.optional-dependencies]\ntest=[\"pytest\"]\n")}}, nil, []string{"pytest"}},
		{"dependency group", Ruff, fstest.MapFS{"pyproject.toml": {Data: []byte("[dependency-groups]\ndev=[\"ruff\", {include-group=\"x\"}]\n")}}, nil, []string{"ruff", "check"}},
		{"tool table", Ruff, fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.ruff]\nline-length=100\n")}}, []string{"src"}, []string{"ruff", "check", "src"}},
		{"config file", Pytest, fstest.MapFS{"pytest.ini": {}}, nil, []string{"pytest"}},
		{"requirements file", Ruff, fstest.MapFS{"requirements-dev.txt": {Data: []byte("# dev\nRuff==0.5\n")}}, nil, []string{"ruff", "check"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.tool.Plan(c.files, c.args)
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Errorf("Plan = %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestToolPlanNotConfigured(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{
		"empty":           {},
		"other tool":      {"requirements.txt": {Data: []byte("pytest-cov==1\n")}},
		"unrelated table": {"pyproject.toml": {Data: []byte("[tool.black]\n")}},
	} {
		_, err := Pytest.Plan(fsys, nil)
		if !errors.Is(err, ErrNotConfigured) || !strings.Contains(err.Error(), Pytest.Enable) {
			t.Errorf("%s: err = %v", name, err)
		}
		un := AsUnsupported(err, "pm", core.CapTest)
		var ue *core.UnsupportedError
		if !errors.As(un, &ue) || ue.PackageManager != "pm" || ue.Reason != "pytest is not configured for this project; "+Pytest.Enable {
			t.Errorf("%s: %v", name, un)
		}
	}
	other := errors.New("other")
	if AsUnsupported(other, "pm", core.CapTest) != other {
		t.Error("other errors pass through")
	}
}

func TestToolPlanReportsBrokenInput(t *testing.T) {
	if _, err := Pytest.Plan(fstest.MapFS{"pyproject.toml": {Data: []byte("[project\n")}}, nil); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("broken pyproject: %v", err)
	}
	broken := fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.upc.scripts]\ntest='a \"b'\n")}}
	if _, err := Pytest.Plan(broken, nil); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("broken script: %v", err)
	}
}

func TestVenvLookup(t *testing.T) {
	linux := fstest.MapFS{".venv/bin/python": {}, ".venv/bin/pytest": {}}
	win := fstest.MapFS{".venv/Scripts/python.exe": {}}
	if p, ok := VenvProgram(linux, "linux", "pytest"); !ok || p != ".venv/bin/pytest" {
		t.Errorf("linux pytest = %q, %v", p, ok)
	}
	if got := Python(win, "windows"); got != ".venv/Scripts/python.exe" {
		t.Errorf("windows python = %q", got)
	}
	if got := Python(fstest.MapFS{}, "linux"); got != "python3" {
		t.Errorf("fallback = %q", got)
	}
	if got := Program(linux, "linux", "ruff"); got != "ruff" {
		t.Errorf("missing program should fall back to PATH, got %q", got)
	}
	if VenvPythonPath("windows") != ".venv/Scripts/python.exe" || VenvPythonPath("linux") != ".venv/bin/python" {
		t.Error("venv python path")
	}
	if !HasVenv(linux) || HasVenv(fstest.MapFS{"x": {}}) {
		t.Error("HasVenv")
	}
}

func TestParsePipListAndSummaries(t *testing.T) {
	pkgs, err := ParsePipList(`[{"name":"a","version":"1","latest_version":"2"},{"name":"b","version":"3"}]`)
	if err != nil || len(pkgs) != 2 || PackageSummary(pkgs) != "a 1\nb 3" {
		t.Fatalf("%+v, %v", pkgs, err)
	}
	stale, summary := OutdatedPackages(pkgs)
	if len(stale) != 1 || summary != "a 1 -> 2" {
		t.Errorf("outdated = %+v %q", stale, summary)
	}
	if got, err := ParsePipList(" "); err != nil || got != nil {
		t.Errorf("empty = %v, %v", got, err)
	}
	if _, err := ParsePipList("x"); err == nil {
		t.Error("garbage must fail")
	}
}

func TestNames(t *testing.T) {
	if got, err := ScriptFileName("tool"); err != nil || got != "tool.py" {
		t.Errorf("%q, %v", got, err)
	}
	for _, bad := range []string{"", "-x", "a/b", `a\b`, "../x"} {
		if _, err := ScriptFileName(bad); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestInvocationNeedsARunner(t *testing.T) {
	if _, err := Begin(context.Background(), core.Request{}, "x"); err == nil {
		t.Error("missing runner must be an error")
	}
	fake := &execx.FakeRunner{}
	inv, err := Begin(execx.WithRunner(context.Background(), fake), core.Request{Dir: "d", DryRun: true}, "tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inv.Run(false, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := inv.RunProgram(true, "other", "b"); err != nil {
		t.Fatal(err)
	}
	res := inv.Result("done", "do it", nil)
	if !reflect.DeepEqual(res.Commands, []string{"tool a", "other b"}) || res.Summary != "dry run, nothing changed: would do it" {
		t.Errorf("result = %+v", res)
	}
	if fake.Calls[0].Dir != "d" || fake.Calls[0].Stream || !fake.Calls[1].Stream {
		t.Errorf("calls = %+v", fake.Calls)
	}
}

func TestProjectNames(t *testing.T) {
	for _, ok := range []string{"demo", "My-Lib", "a_b1"} {
		if err := ValidateProjectName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-x", "1abc", "a/b", "a b", "a.b", ".."} {
		if err := ValidateProjectName(bad); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if got := ModuleName("My-Lib"); got != "my_lib" {
		t.Errorf("ModuleName = %q", got)
	}
}

func TestAppendAppScriptsWritesTheUpcTableThroughTheGivenFS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(path, []byte("[project]\nname = \"demo\""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendAppScripts(fsx.OS{}, dir); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	want := "[project]\nname = \"demo\"\n\n[tool.upc.scripts]\ndev = \"python server.py\"\nstart = \"python server.py\"\n"
	if string(raw) != want {
		t.Errorf("pyproject = %q, want %q", raw, want)
	}
	if err := AppendAppScripts(fsx.OS{}, dir); err == nil {
		t.Error("a second call must refuse an existing [tool.upc] table")
	}
}
