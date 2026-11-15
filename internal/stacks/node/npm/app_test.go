package npm

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitAppArgvAndDirectory(t *testing.T) {
	dir, fake := t.TempDir(), &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"npm init --yes",
		"npm pkg set version=0.1.0 type=module engines.node=>=22.6.0 scripts.build=tsc scripts.dev=node --experimental-strip-types src/main.ts scripts.start=node dist/main.js",
		"npm pkg set private=true --json",
		"npm pkg delete main scripts.test",
		"npm install --save-dev typescript @types/node",
	}
	if got := argv(fake); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	for _, c := range fake.Calls {
		if c.Dir != filepath.Join(dir, "demo") {
			t.Errorf("%s ran in %q", c, c.Dir)
		}
	}
	for _, c := range argv(fake) {
		if strings.Contains(c, "main=") || strings.Contains(c, "types=") || strings.Contains(c, "files") {
			t.Errorf("app manifest must have no library field: %s", c)
		}
	}
}

func TestInitAppFailureRemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	cmd := execx.Cmd{Name: "npm", Args: []string{"install", "--save-dev", "typescript", "@types/node"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo"}, fake); err == nil {
		t.Fatal("want the npm failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); !os.IsNotExist(err) {
		t.Errorf("app directory left behind: %v", err)
	}
}
