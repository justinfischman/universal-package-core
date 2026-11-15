package yarn

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

const manifest0 = `{"name":"demo","version":"1.0.0","dependencies":{"is-odd":"2.0.0","is-even":"0.1.0"},"devDependencies":{"@types/node":"22.0.0"},"scripts":{"build":"tsc -p .","lint":"eslint .","test":"node --test","serve":"node serve.js"}}`

var (
	projectFiles = map[string]string{"package.json": manifest0}
	bareFiles    = map[string]string{"package.json": `{"name":"demo"}`}
	tsFiles      = map[string]string{"package.json": `{"name":"demo"}`, "tsconfig.json": "{}"}
)

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

func newYarn(t *testing.T) *Yarn {
	t.Helper()
	y, err := newFor(fsx.OS{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return y
}

func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	return newYarn(t).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

// argv lists the commands the capability ran, without the `yarn --version`
// flavor probe.
func argv(fake *execx.FakeRunner) []string {
	var out []string
	for _, c := range fake.Calls {
		if s := c.String(); s != "yarn --version" {
			out = append(out, s)
		}
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
	contract.RunPackageManagerSuite(t, newYarn(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newYarn(t).Capabilities()
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
		{"install", core.CapInstall, projectFiles, core.Request{}, []string{"yarn install"}},
		{"install passthrough", core.CapInstall, projectFiles, core.Request{Args: []string{"--frozen-lockfile"}}, []string{"yarn install --frozen-lockfile"}},
		{"add", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod", "left-pad@1"}}, []string{"yarn add zod left-pad@1"}},
		{"add dev", core.CapAdd, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"yarn add -D vitest"}},
		{"add optional", core.CapAdd, projectFiles, core.Request{Packages: []string{"fsevents"}, Group: "optional"}, []string{"yarn add -O fsevents"}},
		{"add peer", core.CapAdd, projectFiles, core.Request{Packages: []string{"react"}, Group: "peer"}, []string{"yarn add -P react"}},
		{"add passthrough", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod"}, Args: []string{"--exact"}}, []string{"yarn add zod --exact"}},
		{"remove", core.CapRemove, projectFiles, core.Request{Packages: []string{"zod"}}, []string{"yarn remove zod"}},
		{"remove dev", core.CapRemove, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"yarn remove vitest"}},
		{"list", core.CapList, projectFiles, core.Request{}, []string{"yarn list --depth=0 --json"}},
		{"update all", core.CapUpdate, projectFiles, core.Request{}, []string{"yarn upgrade"}},
		{"update named", core.CapUpdate, projectFiles, core.Request{Packages: []string{"a", "b"}}, []string{"yarn upgrade a b"}},
		{"outdated", core.CapOutdated, projectFiles, core.Request{}, []string{"yarn outdated --json"}},
		{"audit", core.CapAudit, projectFiles, core.Request{}, []string{"yarn audit"}},
		{"publish", core.CapPublish, projectFiles, core.Request{}, []string{"yarn publish"}},
		{"test", core.CapTest, projectFiles, core.Request{}, []string{"yarn test"}},
		{"test passthrough", core.CapTest, projectFiles, core.Request{Args: []string{"--test-name-pattern=x"}}, []string{"yarn test --test-name-pattern=x"}},
		{"build script", core.CapBuild, projectFiles, core.Request{}, []string{"yarn run build"}},
		{"build compiler", core.CapBuild, tsFiles, core.Request{}, []string{"yarn tsc"}},
		{"lint script", core.CapLint, projectFiles, core.Request{Args: []string{"--fix"}}, []string{"yarn run lint --fix"}},
		{"run script", core.CapRun, projectFiles, core.Request{Script: "serve"}, []string{"yarn run serve"}},
		{"run script args", core.CapRun, projectFiles, core.Request{Script: "serve", Args: []string{"--port", "1"}}, []string{"yarn run serve --port 1"}},
		{"run typescript file", core.CapRun, map[string]string{"package.json": "{}", "tool.ts": ""}, core.Request{Script: "tool.ts", Args: []string{"x"}},
			[]string{"node --experimental-strip-types tool.ts x"}},
		{"run javascript file", core.CapRun, map[string]string{"package.json": "{}", "tool.mjs": ""}, core.Request{Script: "tool.mjs"}, []string{"node tool.mjs"}},
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
		})
	}
}

// Berry (2+) moves upgrade, audit and publish; this is the one berry case.
func TestBerryArgv(t *testing.T) {
	version := execx.Cmd{Name: "yarn", Args: []string{"--version"}}
	cases := map[core.Capability][]string{
		core.CapUpdate:  {"yarn up a"},
		core.CapAudit:   {"yarn npm audit"},
		core.CapPublish: {"yarn npm publish"},
	}
	for c, want := range cases {
		fake := &execx.FakeRunner{Responses: map[string]execx.Response{version.String(): {Output: execx.Output{Stdout: "4.1.0\n"}}}}
		if _, err := call(t, c, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"a"}}, fake); err != nil {
			t.Fatal(err)
		}
		if got := argv(fake); !reflect.DeepEqual(got, want) {
			t.Errorf("%s argv = %q, want %q", c, got, want)
		}
	}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{version.String(): {Output: execx.Output{Stdout: "4.1.0\n"}}}}
	if _, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, fake); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("berry list err = %v, want Unsupported", err)
	}
}

func TestInitArgvAndSeededManifest(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		fake := &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, core.Request{Dir: t.TempDir()}, fake); err != nil {
			t.Fatal(err)
		}
		if got := argv(fake); !reflect.DeepEqual(got, []string{"yarn init --yes"}) {
			t.Errorf("argv = %q", got)
		}
	})
	t.Run("script", func(t *testing.T) {
		dir, fake := t.TempDir(), &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "My_Tool"}, fake); err != nil {
			t.Fatal(err)
		}
		if got := argv(fake); !reflect.DeepEqual(got, []string{"yarn init --yes"}) {
			t.Errorf("argv = %q", got)
		}
		seed := readFile(t, filepath.Join(dir, "package.json"))
		for _, want := range []string{`"name": "my_tool"`, `"private": true`, `"type": "module"`, `"start": "node --experimental-strip-types My_Tool.ts"`, `"node": ">=22.6.0"`} {
			if !strings.Contains(seed, want) {
				t.Errorf("seeded manifest lacks %s:\n%s", want, seed)
			}
		}
		if src := readFile(t, filepath.Join(dir, "My_Tool.ts")); !strings.Contains(src, "console.log") {
			t.Errorf("script source = %q", src)
		}
	})
	t.Run("package", func(t *testing.T) {
		dir, fake := t.TempDir(), &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "package", Name: "demo-lib"}, fake); err != nil {
			t.Fatal(err)
		}
		if got, want := argv(fake), []string{"yarn init --yes", "yarn add --dev typescript @types/node"}; !reflect.DeepEqual(got, want) {
			t.Errorf("argv = %q, want %q", got, want)
		}
		for _, c := range fake.Calls {
			if c.Dir != filepath.Join(dir, "demo-lib") {
				t.Errorf("%s ran in %q", c, c.Dir)
			}
		}
		seed := readFile(t, filepath.Join(dir, "demo-lib", "package.json"))
		for _, want := range []string{`"name": "demo-lib"`, `"main": "dist/index.js"`, `"build": "tsc"`, `"test": "node --test"`, `"node": ">=22.18.0"`} {
			if !strings.Contains(seed, want) {
				t.Errorf("seeded manifest lacks %s:\n%s", want, seed)
			}
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInitFailureCleansUp(t *testing.T) {
	dir := t.TempDir()
	cmd := execx.Cmd{Name: "yarn", Args: []string{"init", "--yes"}}
	fail := func() *execx.FakeRunner {
		return &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "hello"}, fail()); err == nil {
		t.Fatal("want the yarn failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
		t.Errorf("package.json left behind: %v", err)
	}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "package", Name: "demo"}, fail()); err == nil {
		t.Fatal("want the yarn failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); !os.IsNotExist(err) {
		t.Errorf("package directory left behind: %v", err)
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
		"unknown project type": {core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "app", Name: "demo"}},
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

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "yarn", Args: strings.Fields(cmd)[1:]}
	resp := execx.Response{Output: execx.Output{Stdout: out, ExitCode: code}}
	if code != 0 {
		resp.Err = &execx.ExitError{Cmd: c, Code: code}
	}
	return &execx.FakeRunner{Responses: map[string]execx.Response{c.String(): resp}}
}

func TestListParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		files   map[string]string
		want    []core.Package
		summary string
	}{
		// is-number is transitive and @types/node is scoped: only declared names, in name order.
		{"ls_deps.ndjson", projectFiles, []core.Package{{Name: "@types/node", Version: "22.0.0"}, {Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}},
			"@types/node 22.0.0\nis-even 0.1.0\nis-odd 2.0.0"},
		{"ls_empty.ndjson", bareFiles, []core.Package{}, "no dependencies"},
		// declared but not installed: no version
		{"ls_empty.ndjson", projectFiles, []core.Package{{Name: "@types/node"}, {Name: "is-even"}, {Name: "is-odd"}}, "@types/node\nis-even\nis-odd"},
	}
	for _, c := range cases {
		t.Run(c.fixture+"/"+c.summary, func(t *testing.T) {
			fake := scripted("yarn list --depth=0 --json", testdata(t, c.fixture), 0)
			res, err := call(t, core.CapList, core.Request{Dir: fixture(t, c.files)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
			if fake.Calls[len(fake.Calls)-1].Stream {
				t.Error("parsed output must be captured, not streamed")
			}
		})
	}
}

func TestListReportsFailures(t *testing.T) {
	req := core.Request{Dir: fixture(t, projectFiles)}
	_, err := call(t, core.CapList, req, scripted("yarn list --depth=0 --json", testdata(t, "ls_error.ndjson"), 1))
	if err == nil || !strings.Contains(err.Error(), "Expected property name") {
		t.Errorf("err = %v, want yarn's message", err)
	}
	_, err = call(t, core.CapList, req, scripted("yarn list --depth=0 --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapList, req, scripted("yarn list --depth=0 --json", "{not json", 0))
	if err == nil || !strings.Contains(err.Error(), "parse yarn list output") {
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
		// yarn outdated exits 1 when it finds outdated packages: that is a result.
		{"outdated.ndjson", 1, []core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}},
			"is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1"},
		{"outdated_empty.ndjson", 0, []core.Package{}, "all dependencies are up to date"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("yarn outdated --json", testdata(t, c.fixture), c.code))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
		})
	}
}

func TestOutdatedReportsFailures(t *testing.T) {
	req := core.Request{Dir: fixture(t, projectFiles)}
	_, err := call(t, core.CapOutdated, req, scripted("yarn outdated --json", testdata(t, "outdated_error.ndjson"), 1))
	if err == nil || !strings.Contains(err.Error(), `Couldn't find package "nope"`) {
		t.Errorf("err = %v, want yarn's message", err)
	}
	_, err = call(t, core.CapOutdated, req, scripted("yarn outdated --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	for _, bad := range []string{`{"type":"table","data":{"head":["Name"],"body":[]}}`, `{"type":"table","data":{"head":["Package","Current","Latest"],"body":[["a"]]}}`} {
		if _, err = call(t, core.CapOutdated, req, scripted("yarn outdated --json", bad, 0)); err == nil {
			t.Errorf("unexpected shape %s must be an error", bad)
		}
	}
}

func TestDetectRank(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  int
	}{
		{"lock file", fstest.MapFS{"yarn.lock": {}}, rankLockfile},
		{"lock file beats declaration", fstest.MapFS{"yarn.lock": {}, "package.json": {Data: []byte(`{"packageManager":"yarn@4.0.0"}`)}}, rankLockfile},
		{"declaration", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"yarn@1.22.22"}`)}}, rankDeclared},
		{"bare manifest is npm's", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, 0},
		{"other declaration", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}}, 0},
		{"nothing", fstest.MapFS{}, 0},
	}
	y := newYarn(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := y.DetectRank(c.files); got != c.want {
				t.Errorf("rank = %d, want %d", got, c.want)
			}
		})
	}
}

func TestMarkers(t *testing.T) {
	if got := newYarn(t).Markers(); !reflect.DeepEqual(got, []string{"yarn.lock", "package.json"}) && !reflect.DeepEqual(got, []string{"package.json", "yarn.lock"}) {
		t.Errorf("markers = %v", got)
	}
}

func TestDiagnose(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"locked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "yarn.lock": {}}, nil},
		{"unlocked", fstest.MapFS{"package.json": {Data: []byte(`{}`)}}, []string{"yarn.lock is missing"}},
		{"other manager declared", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"npm@11.0.0"}`)}, "yarn.lock": {}}, []string{`packageManager "npm@11.0.0"`}},
		{"no manifest", fstest.MapFS{"yarn.lock": {}}, nil},
	}
	y := newYarn(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, f := range y.Diagnose(c.files) {
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
