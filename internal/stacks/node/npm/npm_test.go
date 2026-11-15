package npm

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
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

const manifest = `{"name":"demo","version":"1.0.0","scripts":{"build":"tsc -p .","lint":"eslint .","test":"node --test","serve":"node serve.js"}}`

var (
	projectFiles = map[string]string{"package.json": manifest}
	bareFiles    = map[string]string{"package.json": `{"name":"demo"}`}
	tsFiles      = map[string]string{"package.json": `{"name":"demo"}`, "tsconfig.json": "{}"}
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

func newNPM(t *testing.T) *NPM {
	t.Helper()
	n, err := newFor(fsx.OS{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// call executes a capability against a FakeRunner.
func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	return newNPM(t).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func testdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestContractSuite(t *testing.T) {
	contract.RunPackageManagerSuite(t, newNPM(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newNPM(t).Capabilities()
	want := map[core.Capability]core.Level{
		core.CapInit: core.LevelNative, core.CapInstall: core.LevelNative, core.CapAdd: core.LevelNative,
		core.CapRemove: core.LevelNative, core.CapList: core.LevelNative, core.CapUpdate: core.LevelNative,
		core.CapOutdated: core.LevelNative, core.CapAudit: core.LevelNative, core.CapRun: core.LevelNative,
		core.CapTest: core.LevelNative, core.CapPublish: core.LevelNative,
		core.CapBuild: core.LevelFilled, core.CapLint: core.LevelFilled,
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
		{"install", core.CapInstall, projectFiles, core.Request{}, []string{"npm install"}},
		{"install passthrough", core.CapInstall, projectFiles, core.Request{Args: []string{"--ignore-scripts"}}, []string{"npm install --ignore-scripts"}},
		{"add", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod", "left-pad@1"}}, []string{"npm install zod left-pad@1"}},
		{"add dev", core.CapAdd, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"npm install -D vitest"}},
		{"add optional", core.CapAdd, projectFiles, core.Request{Packages: []string{"fsevents"}, Group: "optional"}, []string{"npm install -O fsevents"}},
		{"add peer", core.CapAdd, projectFiles, core.Request{Packages: []string{"react"}, Group: "peer"}, []string{"npm install --save-peer react"}},
		{"add passthrough", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod"}, Args: []string{"--save-exact"}}, []string{"npm install zod --save-exact"}},
		{"remove", core.CapRemove, projectFiles, core.Request{Packages: []string{"zod"}}, []string{"npm uninstall zod"}},
		{"remove dev", core.CapRemove, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"npm uninstall -D vitest"}},
		{"list", core.CapList, projectFiles, core.Request{}, []string{"npm ls --depth=0 --json"}},
		{"update all", core.CapUpdate, projectFiles, core.Request{}, []string{"npm update"}},
		{"update named", core.CapUpdate, projectFiles, core.Request{Packages: []string{"a", "b"}}, []string{"npm update a b"}},
		{"outdated", core.CapOutdated, projectFiles, core.Request{}, []string{"npm outdated --json"}},
		{"audit", core.CapAudit, projectFiles, core.Request{}, []string{"npm audit"}},
		{"audit passthrough", core.CapAudit, projectFiles, core.Request{Args: []string{"--omit=dev"}}, []string{"npm audit --omit=dev"}},
		{"publish", core.CapPublish, projectFiles, core.Request{}, []string{"npm publish"}},
		{"test", core.CapTest, projectFiles, core.Request{}, []string{"npm test"}},
		{"test passthrough", core.CapTest, projectFiles, core.Request{Args: []string{"--test-name-pattern=x"}}, []string{"npm test -- --test-name-pattern=x"}},
		{"build script", core.CapBuild, projectFiles, core.Request{}, []string{"npm run build"}},
		{"build script passthrough", core.CapBuild, projectFiles, core.Request{Args: []string{"--noEmit"}}, []string{"npm run build -- --noEmit"}},
		{"build compiler", core.CapBuild, tsFiles, core.Request{}, []string{"npm exec --no -- tsc"}},
		{"lint script", core.CapLint, projectFiles, core.Request{Args: []string{"--fix"}}, []string{"npm run lint -- --fix"}},
		{"run script", core.CapRun, projectFiles, core.Request{Script: "serve"}, []string{"npm run serve"}},
		{"run script args", core.CapRun, projectFiles, core.Request{Script: "serve", Args: []string{"--port", "1"}}, []string{"npm run serve -- --port 1"}},
		{"run typescript file", core.CapRun, map[string]string{"package.json": "{}", "tool.ts": ""}, core.Request{Script: "tool.ts", Args: []string{"x"}},
			[]string{"node --experimental-strip-types tool.ts x"}},
		{"run javascript file", core.CapRun, map[string]string{"package.json": "{}", "tool.mjs": ""}, core.Request{Script: "tool.mjs"}, []string{"node tool.mjs"}},
		{"file wins over script", core.CapRun, map[string]string{"package.json": `{"scripts":{"tool.js":"echo"}}`, "tool.js": ""}, core.Request{Script: "tool.js"}, []string{"node tool.js"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, c.files)
			fake := &execx.FakeRunner{}
			res, err := call(t, c.cap, c.req, fake)
			if err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, c.want) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if !reflect.DeepEqual(res.Commands, c.want) {
				t.Errorf("result commands = %q", res.Commands)
			}
			if fake.Calls[0].Dir != c.req.Dir {
				t.Errorf("dir = %q, want %q", fake.Calls[0].Dir, c.req.Dir)
			}
		})
	}
}

func TestInitArgv(t *testing.T) {
	cases := []struct {
		name string
		req  core.Request
		want []string
	}{
		{"plain", core.Request{}, []string{"npm init --yes"}},
		{"script", core.Request{ProjectType: "script", Script: "hello"}, []string{
			"npm init --yes",
			"npm pkg set version=0.1.0 type=module engines.node=>=22.6.0 scripts.start=node --experimental-strip-types hello.ts",
			"npm pkg set private=true --json",
			"npm pkg delete main scripts.test",
		}},
		{"javascript script", core.Request{ProjectType: "script", Script: "tool.js"}, []string{
			"npm init --yes",
			"npm pkg set version=0.1.0 type=module engines.node=>=22.6.0 scripts.start=node tool.js",
			"npm pkg set private=true --json",
			"npm pkg delete main scripts.test",
		}},
		{"package", core.Request{ProjectType: "package", Name: "demo-lib"}, []string{
			"npm init --yes",
			"npm pkg set version=0.1.0 type=module main=dist/index.js types=dist/index.d.ts files[]=dist scripts.build=tsc scripts.test=node --test engines.node=>=22.18.0",
			"npm install --save-dev typescript @types/node",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = t.TempDir()
			fake := &execx.FakeRunner{}
			if _, err := call(t, core.CapInit, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, c.want) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
		})
	}
}

func TestInitScriptWritesSourceAndSeededName(t *testing.T) {
	dir := t.TempDir()
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "My_Tool"}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, "My_Tool.ts"))
	if err != nil || !strings.Contains(string(src), "console.log") {
		t.Errorf("script source = %q, %v", src, err)
	}
	seed, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil || !strings.Contains(string(seed), `"name": "my_tool"`) {
		t.Errorf("seeded manifest = %q, %v", seed, err)
	}
}

func TestInitScriptFailureLeavesNoManifest(t *testing.T) {
	dir := t.TempDir()
	cmd := execx.Cmd{Name: "npm", Args: []string{"init", "--yes"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "hello"}, fake); err == nil {
		t.Fatal("want the npm failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
		t.Errorf("package.json left behind: %v", err)
	}
}

func TestInitPackageFailureRemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	cmd := execx.Cmd{Name: "npm", Args: []string{"install", "--save-dev", "typescript", "@types/node"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "package", Name: "demo"}, fake); err == nil {
		t.Fatal("want the npm failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); !os.IsNotExist(err) {
		t.Errorf("package directory left behind: %v", err)
	}
}

func TestInitPackageRunsInsideTheNewDirectory(t *testing.T) {
	dir := t.TempDir()
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "package", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	for _, c := range fake.Calls {
		if c.Dir != filepath.Join(dir, "demo") {
			t.Errorf("%s ran in %q", c, c.Dir)
		}
	}
}

func TestInitDryRunChangesNothing(t *testing.T) {
	for _, req := range []core.Request{
		{ProjectType: "script", Script: "hello"},
		{ProjectType: "package", Name: "demo"},
	} {
		dir := t.TempDir()
		req.Dir, req.DryRun = dir, true
		res, err := call(t, core.CapInit, req, &execx.FakeRunner{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(res.Summary, "dry run, nothing changed") || len(res.Commands) == 0 {
			t.Errorf("result = %+v", res)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("dry run created %v", entries)
		}
	}
}

func TestInitRejectsBadInput(t *testing.T) {
	existing := fixture(t, map[string]string{"hello.ts": "", "demo/x": ""})
	cases := map[string]core.Request{
		"script flag-like":     {Dir: t.TempDir(), ProjectType: "script", Script: "-x"},
		"script path":          {Dir: t.TempDir(), ProjectType: "script", Script: "a/b.ts"},
		"script escape":        {Dir: t.TempDir(), ProjectType: "script", Script: "../x"},
		"script extension":     {Dir: t.TempDir(), ProjectType: "script", Script: "x.py"},
		"script no name":       {Dir: t.TempDir(), ProjectType: "script", Script: "..."},
		"script exists":        {Dir: existing, ProjectType: "script", Script: "hello"},
		"package upper case":   {Dir: t.TempDir(), ProjectType: "package", Name: "Demo"},
		"package flag-like":    {Dir: t.TempDir(), ProjectType: "package", Name: "-x"},
		"package exists":       {Dir: existing, ProjectType: "package", Name: "demo"},
		"unknown project type": {Dir: t.TempDir(), ProjectType: "app", Name: "demo"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, core.CapInit, req, fake); !errors.Is(err, core.ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
			if len(fake.Calls) != 0 {
				t.Errorf("ran %v despite invalid input", argv(fake))
			}
		})
	}
}

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

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "npm", Args: strings.Fields(cmd)[1:]}
	resp := execx.Response{Output: execx.Output{Stdout: out, ExitCode: code}}
	if code != 0 {
		resp.Err = &execx.ExitError{Cmd: c, Code: code}
	}
	return &execx.FakeRunner{Responses: map[string]execx.Response{c.String(): resp}}
}

func TestListParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		code    int
		want    []core.Package
		summary string
	}{
		{"ls_deps.json", 0, []core.Package{{Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}}, "is-even 0.1.0\nis-odd 2.0.0"},
		{"ls_empty.json", 0, []core.Package{}, "no dependencies"},
		{"ls_missing.json", 1, []core.Package{{Name: "is-odd", Version: "2.0.0"}, {Name: "zod"}}, "is-odd 2.0.0\nzod"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("npm ls --depth=0 --json", testdata(t, c.fixture), c.code)
			res, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
			if fake.Calls[0].Stream {
				t.Error("parsed output must be captured, not streamed")
			}
		})
	}
}

func TestListReportsNpmErrors(t *testing.T) {
	_, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", testdata(t, "ls_error.json"), 1))
	if err == nil || !strings.Contains(err.Error(), "EJSONPARSE") {
		t.Errorf("err = %v, want the npm error code", err)
	}
	_, err = call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", "{not json", 0))
	if err == nil || !strings.Contains(err.Error(), "parse npm ls output") {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestOutdatedParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		code    int
		want    []core.Package
		summary string
	}{
		// npm outdated exits 1 when it finds outdated packages: that is a result, not a failure.
		{"outdated.json", 1, []core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}},
			"is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1"},
		{"outdated_empty.json", 0, []core.Package{}, "all dependencies are up to date"},
		{"outdated_multi.json", 1, []core.Package{{Name: "left-pad", Version: "1.0.0", Latest: "1.3.0"}, {Name: "not-installed", Latest: "2.0.0"}},
			"left-pad 1.0.0 -> 1.3.0\nnot-installed  -> 2.0.0"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("npm outdated --json", testdata(t, c.fixture), c.code)
			res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
		})
	}
}

func TestOutdatedEmptyOutputMeansUpToDate(t *testing.T) {
	res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "", 0))
	if err != nil || res.Summary != "all dependencies are up to date" {
		t.Errorf("result = %+v, err = %v", res, err)
	}
}

func TestOutdatedReportsFailures(t *testing.T) {
	_, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", testdata(t, "outdated_error.json"), 1))
	if err == nil || !strings.Contains(err.Error(), "ENOTFOUND") {
		t.Errorf("err = %v, want the npm error code", err)
	}
	_, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "[1]", 0))
	if err == nil {
		t.Error("unexpected shape must be an error")
	}
}

func TestDetectRank(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  int
	}{
		{"lock file", fstest.MapFS{"package-lock.json": {}}, rankLockfile},
		{"lock file beats manifest", fstest.MapFS{"package-lock.json": {}, "package.json": {}}, rankLockfile},
		{"manifest", fstest.MapFS{"package.json": {}}, rankManifest},
		{"nothing", fstest.MapFS{}, 0},
		{"tsconfig alone is not a manager marker", fstest.MapFS{"tsconfig.json": {}}, 0},
	}
	n := newNPM(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := n.DetectRank(c.files); got != c.want {
				t.Errorf("rank = %d, want %d", got, c.want)
			}
			if n.Detect(c.files) != (c.want > 0) {
				t.Errorf("Detect disagrees with rank %d", c.want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	if got := newNPM(t).Markers(); !reflect.DeepEqual(got, []string{"package-lock.json", "package.json"}) {
		t.Errorf("markers = %v", got)
	}
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"locked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "package-lock.json": {}}, nil},
		{"unlocked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, []string{"package-lock.json is missing"}},
		{"other manager declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"yarn@4.0.0"}`)}, "package-lock.json": {}},
			[]string{`packageManager "yarn@4.0.0"`}},
		{"npm declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}, "package-lock.json": {}}, nil},
		{"no manifest", fstest.MapFS{"package-lock.json": {}}, nil},
	}
	n := newNPM(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, f := range n.Diagnose(c.files) {
				got = append(got, f.Message)
			}
			if len(got) != len(c.want) {
				t.Fatalf("findings = %q, want %q", got, c.want)
			}
			for i := range got {
				if !strings.Contains(got[i], c.want[i]) {
					t.Errorf("finding %q does not mention %q", got[i], c.want[i])
				}
			}
		})
	}
}
