package npm

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
