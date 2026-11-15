package npm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestDependencyCommandsRejectBadInput(t *testing.T) {
	dir := fixture(t, projectFiles)
	cases := []struct {
		name string
		cap  core.Capability
		req  core.Request
	}{
		{"add no packages", core.CapAdd, core.Request{Dir: dir}},
		{"add flag-like package", core.CapAdd, core.Request{Dir: dir, Packages: []string{"--registry=evil"}}},
		{"add empty package", core.CapAdd, core.Request{Dir: dir, Packages: []string{" "}}},
		{"add unknown group", core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "docs"}},
		{"add flag-like group", core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "--bad"}},
		{"remove no packages", core.CapRemove, core.Request{Dir: dir}},
		{"update flag-like package", core.CapUpdate, core.Request{Dir: dir, Packages: []string{"--force"}}},
		{"install group", core.CapInstall, core.Request{Dir: dir, Group: "dev"}},
		{"run unknown script", core.CapRun, core.Request{Dir: dir, Script: "nope"}},
		{"run flag-like script", core.CapRun, core.Request{Dir: dir, Script: "--help"}},
		{"run escaping file", core.CapRun, core.Request{Dir: dir, Script: "../x.js"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); !errors.Is(err, core.ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("ran %v despite invalid input", argv(fake))
			}
		})
	}
}

func TestEveryCommandNeedsAManifest(t *testing.T) {
	for _, c := range []core.Capability{core.CapInstall, core.CapAdd, core.CapRemove, core.CapList, core.CapUpdate,
		core.CapOutdated, core.CapAudit, core.CapRun, core.CapBuild, core.CapTest, core.CapLint, core.CapPublish} {
		t.Run(string(c), func(t *testing.T) {
			fake := &execx.FakeRunner{}
			req := core.Request{Dir: t.TempDir(), Packages: []string{"x"}, Script: "x"}
			if _, err := call(t, c, req, fake); !errors.Is(err, core.ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("ran %v without a package.json", argv(fake))
			}
		})
	}
}

func TestFillsWithoutConfigurationAreUnsupportedWithGuidance(t *testing.T) {
	cases := []struct {
		cap   core.Capability
		files map[string]string
		hint  string
	}{
		{core.CapBuild, bareFiles, "tsconfig.json"},
		{core.CapLint, bareFiles, "`lint` script"},
		{core.CapLint, tsFiles, "`lint` script"},
	}
	for _, c := range cases {
		t.Run(string(c.cap), func(t *testing.T) {
			fake := &execx.FakeRunner{}
			_, err := call(t, c.cap, core.Request{Dir: fixture(t, c.files)}, fake)
			if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), c.hint) {
				t.Errorf("err = %v, want Unsupported mentioning %q", err, c.hint)
			}
			if core.ExitCode(err) != core.ExitUnsupported || len(fake.Calls) != 0 {
				t.Errorf("exit = %d, ran %v", core.ExitCode(err), argv(fake))
			}
		})
	}
}

func TestBrokenManifestIsReported(t *testing.T) {
	_, err := call(t, core.CapBuild, core.Request{Dir: fixture(t, map[string]string{"package.json": "{bad"})}, &execx.FakeRunner{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

func TestDryRunReportsPlan(t *testing.T) {
	res, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"zod"}, DryRun: true}, &execx.FakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "dry run, nothing changed: would add zod"; res.Summary != want || len(res.Commands) != 1 {
		t.Errorf("result = %+v, want summary %q and one command", res, want)
	}
}

func TestNonZeroExitIsReturnedAsExitError(t *testing.T) {
	cmd := execx.Cmd{Name: "npm", Args: []string{"install", "nope"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1, Stderr: "404"}},
	}}
	_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"nope"}}, fake)
	var exit *execx.ExitError
	if !errors.As(err, &exit) || exit.Code != 1 || core.ExitCode(err) != core.ExitFailure {
		t.Fatalf("err = %v, want ExitError code 1", err)
	}
}

func TestMissingToolCarriesInstallHint(t *testing.T) {
	cmd := execx.Cmd{Name: "npm", Args: []string{"install"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &core.ToolMissingError{Tool: "npm"}}}}
	_, err := call(t, core.CapInstall, core.Request{Dir: fixture(t, projectFiles)}, fake)
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) || missing.InstallHint == "" {
		t.Fatalf("err = %v, want a tool-missing error with an install hint", err)
	}
	if core.ExitCode(err) != core.ExitToolMissing {
		t.Errorf("exit = %d", core.ExitCode(err))
	}
}

func TestToolInstallCommandPerPlatform(t *testing.T) {
	if got := tool("darwin").InstallCmd; !reflect.DeepEqual(got, []string{"brew", "install", "node"}) {
		t.Errorf("darwin install = %v", got)
	}
	for _, goos := range []string{"linux", "windows"} {
		if len(tool(goos).InstallCmd) != 0 || tool(goos).InstallHint == "" {
			t.Errorf("%s should only hint: %+v", goos, tool(goos))
		}
	}
}
