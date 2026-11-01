package uv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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

const scriptSource = "# /// script\n# dependencies = []\n# ///\nprint('hi')\n"

var (
	projectFiles = map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n"}
	scriptFiles  = map[string]string{"hello.py": scriptSource}
)

func newUV(t *testing.T) *UV {
	t.Helper()
	u, err := newFor("linux")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// call executes a capability against a FakeRunner and returns the recorded commands.
func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	ctx := execx.WithRunner(context.Background(), fake)
	return newUV(t).Execute(ctx, c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func TestAddRoutesExactArgv(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		req   core.Request
		want  string
	}{
		{"project", projectFiles, core.Request{Packages: []string{"requests", "rich>=13"}}, "uv add requests rich>=13"},
		{"dev flag", projectFiles, core.Request{Packages: []string{"pytest"}, Group: "dev"}, "uv add --dev pytest"},
		{"named group", projectFiles, core.Request{Packages: []string{"sphinx"}, Group: "docs"}, "uv add --group docs sphinx"},
		{"auto-detected script", scriptFiles, core.Request{Packages: []string{"rich"}}, "uv add --script hello.py rich"},
		{"explicit script", scriptFiles, core.Request{Packages: []string{"rich"}, Script: "hello.py"}, "uv add --script hello.py rich"},
		{"passthrough args", projectFiles, core.Request{Packages: []string{"rich"}, Args: []string{"--no-sync"}}, "uv add rich --no-sync"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, c.files)
			fake := &execx.FakeRunner{}
			res, err := call(t, core.CapAdd, c.req, fake)
			if err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if !reflect.DeepEqual(res.Commands, []string{c.want}) {
				t.Errorf("result commands = %q", res.Commands)
			}
			if fake.Calls[0].Dir != c.req.Dir || !fake.Calls[0].Stream {
				t.Errorf("call = %+v, want dir %q and streaming", fake.Calls[0], c.req.Dir)
			}
		})
	}
}

func TestAddDryRunReportsPlan(t *testing.T) {
	fake := &execx.FakeRunner{}
	res, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"rich"}, DryRun: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	want := "dry run, nothing changed: would add rich"
	if res.Summary != want || len(res.Commands) != 1 {
		t.Errorf("result = %+v, want summary %q and one command", res, want)
	}
}

func TestAddRejectsBadInput(t *testing.T) {
	dir := fixture(t, projectFiles)
	cases := map[string]core.Request{
		"no packages":     {Dir: dir},
		"flag-like pkg":   {Dir: dir, Packages: []string{"--index-url=evil"}},
		"empty pkg":       {Dir: dir, Packages: []string{" "}},
		"flag-like group": {Dir: dir, Packages: []string{"x"}, Group: "--bad"},
		"missing script":  {Dir: dir, Packages: []string{"x"}, Script: "nope.py"},
		"escaping script": {Dir: dir, Packages: []string{"x"}, Script: "../x.py"},
		"empty dir":       {Dir: t.TempDir(), Packages: []string{"x"}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			_, err := call(t, core.CapAdd, req, fake)
			if !errors.Is(err, core.ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("ran %v despite invalid input", argv(fake))
			}
		})
	}
}

func TestAddOnlyRequirementsFileIsExplained(t *testing.T) {
	dir := fixture(t, map[string]string{"requirements.txt": "rich\n"})
	_, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}}, &execx.FakeRunner{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v", err)
	}
}

func TestAddSeveralScriptsNeedsChoice(t *testing.T) {
	dir := fixture(t, map[string]string{"a.py": scriptSource, "b.py": scriptSource})
	_, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}}, &execx.FakeRunner{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v, want a request to pick with --script", err)
	}
}

func TestNonZeroExitIsReturnedAsExitError(t *testing.T) {
	dir := fixture(t, projectFiles)
	cmd := execx.Cmd{Name: "uv", Args: []string{"add", "nope"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1, Stderr: "package not found"}},
	}}
	_, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"nope"}}, fake)
	var exit *execx.ExitError
	if !errors.As(err, &exit) || exit.Code != 1 {
		t.Fatalf("err = %v, want ExitError code 1", err)
	}
	if core.ExitCode(err) != core.ExitFailure {
		t.Errorf("exit code = %d", core.ExitCode(err))
	}
}
