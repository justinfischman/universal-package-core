package pnpm

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

func newPnpm(t *testing.T) *Pnpm {
	t.Helper()
	y, err := newFor(fsx.OS{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return y
}

func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	return newPnpm(t).Execute(execx.WithRunner(context.Background(), fake), c, req)
}

// argv lists the commands the capability ran, without the `pnpm --version`
// flavor probe.
func argv(fake *execx.FakeRunner) []string {
	var out []string
	for _, c := range fake.Calls {
		if s := c.String(); s != "pnpm --version" {
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
	contract.RunPackageManagerSuite(t, newPnpm(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newPnpm(t).Capabilities()
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
		{"install", core.CapInstall, projectFiles, core.Request{}, []string{"pnpm install"}},
		{"install passthrough", core.CapInstall, projectFiles, core.Request{Args: []string{"--frozen-lockfile"}}, []string{"pnpm install --frozen-lockfile"}},
		{"add", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod", "left-pad@1"}}, []string{"pnpm add zod left-pad@1"}},
		{"add dev", core.CapAdd, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"pnpm add -D vitest"}},
		{"add optional", core.CapAdd, projectFiles, core.Request{Packages: []string{"fsevents"}, Group: "optional"}, []string{"pnpm add -O fsevents"}},
		{"add peer", core.CapAdd, projectFiles, core.Request{Packages: []string{"react"}, Group: "peer"}, []string{"pnpm add --save-peer react"}},
		{"add passthrough", core.CapAdd, projectFiles, core.Request{Packages: []string{"zod"}, Args: []string{"--exact"}}, []string{"pnpm add zod --exact"}},
		{"remove", core.CapRemove, projectFiles, core.Request{Packages: []string{"zod"}}, []string{"pnpm remove zod"}},
		{"remove dev", core.CapRemove, projectFiles, core.Request{Packages: []string{"vitest"}, Group: "dev"}, []string{"pnpm remove vitest"}},
		{"list", core.CapList, projectFiles, core.Request{}, []string{"pnpm ls --depth=0 --json"}},
		{"update all", core.CapUpdate, projectFiles, core.Request{}, []string{"pnpm update"}},
		{"update named", core.CapUpdate, projectFiles, core.Request{Packages: []string{"a", "b"}}, []string{"pnpm update a b"}},
		{"outdated", core.CapOutdated, projectFiles, core.Request{}, []string{"pnpm outdated --format json"}},
		{"audit", core.CapAudit, projectFiles, core.Request{}, []string{"pnpm audit"}},
		{"publish", core.CapPublish, projectFiles, core.Request{}, []string{"pnpm publish"}},
		{"test", core.CapTest, projectFiles, core.Request{}, []string{"pnpm test"}},
		{"test passthrough", core.CapTest, projectFiles, core.Request{Args: []string{"--test-name-pattern=x"}}, []string{"pnpm test --test-name-pattern=x"}},
		{"build script", core.CapBuild, projectFiles, core.Request{}, []string{"pnpm run build"}},
		{"build compiler", core.CapBuild, tsFiles, core.Request{}, []string{"pnpm exec tsc"}},
		{"lint script", core.CapLint, projectFiles, core.Request{Args: []string{"--fix"}}, []string{"pnpm run lint --fix"}},
		{"run script", core.CapRun, projectFiles, core.Request{Script: "serve"}, []string{"pnpm run serve"}},
		{"run script args", core.CapRun, projectFiles, core.Request{Script: "serve", Args: []string{"--port", "1"}}, []string{"pnpm run serve --port 1"}},
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
