package yarn

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInvalidInputRunsNothing(t *testing.T) {
	dir := fixture(t, projectFiles)
	existing := fixture(t, map[string]string{"hello.ts": "", "demo/x": ""})
	cases := map[string]struct {
		cap core.Capability
		req core.Request
	}{
		"add no packages":      {core.CapAdd, core.Request{Dir: dir}},
		"add flag-like":        {core.CapAdd, core.Request{Dir: dir, Packages: []string{"--registry=evil"}}},
		"add unknown group":    {core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "docs"}},
		"remove no packages":   {core.CapRemove, core.Request{Dir: dir}},
		"remove unknown group": {core.CapRemove, core.Request{Dir: dir, Packages: []string{"x"}, Group: "docs"}},
		"update flag-like":     {core.CapUpdate, core.Request{Dir: dir, Packages: []string{"--force"}}},
		"install group":        {core.CapInstall, core.Request{Dir: dir, Group: "dev"}},
		"run unknown script":   {core.CapRun, core.Request{Dir: dir, Script: "nope"}},
		"run flag-like script": {core.CapRun, core.Request{Dir: dir, Script: "--help"}},
		"script flag-like":     {core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "script", Script: "-x"}},
		"script exists":        {core.CapInit, core.Request{Dir: existing, ProjectType: "script", Script: "hello"}},
		"package upper case":   {core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "package", Name: "Demo"}},
		"package exists":       {core.CapInit, core.Request{Dir: existing, ProjectType: "package", Name: "demo"}},
		"app upper case":       {core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "app", Name: "Demo"}},
		"app exists":           {core.CapInit, core.Request{Dir: existing, ProjectType: "app", Name: "demo"}},
		"unknown project type": {core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "service", Name: "demo"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
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
	}
	for _, c := range cases {
		t.Run(string(c.cap), func(t *testing.T) {
			fake := &execx.FakeRunner{}
			_, err := call(t, c.cap, core.Request{Dir: fixture(t, c.files)}, fake)
			if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), c.hint) || len(fake.Calls) != 0 {
				t.Errorf("err = %v, ran %v", err, argv(fake))
			}
		})
	}
}

func TestDryRunReportsPlan(t *testing.T) {
	res, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"zod"}, DryRun: true}, &execx.FakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "dry run, nothing changed: would add zod"; res.Summary != want || len(res.Commands) != 1 {
		t.Errorf("result = %+v", res)
	}
}

func TestNonZeroExitIsReturnedAsExitError(t *testing.T) {
	cmd := execx.Cmd{Name: "yarn", Args: []string{"add", "nope"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"nope"}}, fake)
	var exit *execx.ExitError
	if !errors.As(err, &exit) || exit.Code != 1 || core.ExitCode(err) != core.ExitFailure {
		t.Fatalf("err = %v, want ExitError code 1", err)
	}
}

func TestMissingToolCarriesInstallHint(t *testing.T) {
	cmd := execx.Cmd{Name: "yarn", Args: []string{"install"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &core.ToolMissingError{Tool: "yarn"}}}}
	_, err := call(t, core.CapInstall, core.Request{Dir: fixture(t, projectFiles)}, fake)
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) || missing.InstallHint == "" || core.ExitCode(err) != core.ExitToolMissing {
		t.Fatalf("err = %v, want a tool-missing error with an install hint", err)
	}
	// The flavor probe is the first command of update/audit/publish/list: a missing tool surfaces there too.
	probe := execx.Cmd{Name: "yarn", Args: []string{"--version"}}
	fake = &execx.FakeRunner{Responses: map[string]execx.Response{probe.String(): {Err: &core.ToolMissingError{Tool: "yarn"}}}}
	_, err = call(t, core.CapAudit, core.Request{Dir: fixture(t, projectFiles)}, fake)
	if !errors.As(err, &missing) {
		t.Errorf("audit err = %v, want tool missing", err)
	}
}

func TestToolInstallCommandPerPlatform(t *testing.T) {
	if got := tool("darwin").InstallCmd; !reflect.DeepEqual(got, []string{"brew", "install", "yarn"}) {
		t.Errorf("darwin install = %v", got)
	}
	for _, goos := range []string{"linux", "windows"} {
		if len(tool(goos).InstallCmd) != 0 || tool(goos).InstallHint == "" {
			t.Errorf("%s should only hint: %+v", goos, tool(goos))
		}
	}
}
