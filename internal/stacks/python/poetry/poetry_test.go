package poetry

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
)

const pyproject = "[tool.poetry]\nname = \"demo\"\nversion = \"0.1.0\"\n"

var projectFiles = map[string]string{"pyproject.toml": pyproject}

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

func newPoetry(t *testing.T) *Poetry {
	t.Helper()
	p, err := newFor(fsx.OS{}, "linux")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	return newPoetry(t).Execute(execx.WithRunner(t.Context(), fake), c, req)
}

func argv(fake *execx.FakeRunner) []string {
	out := make([]string, len(fake.Calls))
	for i, c := range fake.Calls {
		out[i] = c.String()
	}
	return out
}

func TestContractSuite(t *testing.T) {
	contract.RunPackageManagerSuite(t, newPoetry(t))
}

func TestEveryCapabilityIsDeclared(t *testing.T) {
	caps := newPoetry(t).Capabilities()
	want := map[core.Capability]core.Level{
		core.CapInit: core.LevelNative, core.CapInstall: core.LevelNative, core.CapAdd: core.LevelNative,
		core.CapRemove: core.LevelNative, core.CapList: core.LevelNative, core.CapUpdate: core.LevelNative,
		core.CapOutdated: core.LevelNative, core.CapRun: core.LevelNative, core.CapBuild: core.LevelNative,
		core.CapPublish: core.LevelNative,
		core.CapTest:    core.LevelFilled, core.CapLint: core.LevelFilled, core.CapAudit: core.LevelFilled,
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
		name string
		cap  core.Capability
		req  core.Request
		want string
	}{
		{"init", core.CapInit, core.Request{}, "poetry init --no-interaction"},
		{"install", core.CapInstall, core.Request{}, "poetry install"},
		{"install group", core.CapInstall, core.Request{Group: "docs"}, "poetry install --with docs"},
		{"install env groups", core.CapInstall, core.Request{Groups: []string{"docs", "test"}}, "poetry install --with docs,test"},
		{"add", core.CapAdd, core.Request{Packages: []string{"requests", "rich>=13"}}, "poetry add requests rich>=13"},
		{"add dev", core.CapAdd, core.Request{Packages: []string{"pytest"}, Group: "dev"}, "poetry add --group dev pytest"},
		{"add passthrough", core.CapAdd, core.Request{Packages: []string{"rich"}, Args: []string{"--lock"}}, "poetry add rich --lock"},
		{"remove", core.CapRemove, core.Request{Packages: []string{"rich"}, Group: "docs"}, "poetry remove --group docs rich"},
		{"list", core.CapList, core.Request{}, "poetry show --top-level"},
		{"outdated", core.CapOutdated, core.Request{}, "poetry show --top-level --outdated"},
		{"update all", core.CapUpdate, core.Request{}, "poetry update"},
		{"update named", core.CapUpdate, core.Request{Packages: []string{"a", "b"}}, "poetry update a b"},
		{"build", core.CapBuild, core.Request{Args: []string{"--format", "wheel"}}, "poetry build --format wheel"},
		{"publish", core.CapPublish, core.Request{}, "poetry publish"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.req.Dir = fixture(t, projectFiles)
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			if got := argv(fake); !reflect.DeepEqual(got, []string{c.want}) {
				t.Errorf("argv = %q, want %q", got, c.want)
			}
			if fake.Calls[0].Dir != c.req.Dir {
				t.Errorf("dir = %q, want %q", fake.Calls[0].Dir, c.req.Dir)
			}
		})
	}
}

func TestDryRunStillRoutesButSummaryPlansOnly(t *testing.T) {
	for _, c := range []core.Capability{core.CapInstall, core.CapList, core.CapOutdated, core.CapUpdate, core.CapBuild} {
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
