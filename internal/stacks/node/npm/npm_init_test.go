package npm

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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
		"app upper case":       {Dir: t.TempDir(), ProjectType: "app", Name: "Demo"},
		"app exists":           {Dir: existing, ProjectType: "app", Name: "demo"},
		"unknown project type": {Dir: t.TempDir(), ProjectType: "service", Name: "demo"},
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
