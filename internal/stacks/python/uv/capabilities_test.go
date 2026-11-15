package uv

import (
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
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
		{"install env groups", core.CapInstall, projectFiles, core.Request{Group: "dev", Groups: []string{"docs", "dev"}}, []string{"uv sync --group dev --group docs"}},
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
