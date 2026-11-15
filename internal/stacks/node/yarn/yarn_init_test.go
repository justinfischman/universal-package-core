package yarn

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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
		{ProjectType: "app", Name: "demo"},
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
