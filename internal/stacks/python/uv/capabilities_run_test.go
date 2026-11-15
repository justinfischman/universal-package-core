package uv

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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
