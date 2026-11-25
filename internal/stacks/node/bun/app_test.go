package bun

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitAppSeedsApplicationManifest(t *testing.T) {
	dir, fake := t.TempDir(), &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo-app"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"bun add --dev typescript @types/node"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	for _, c := range fake.Calls {
		if c.Dir != filepath.Join(dir, "demo-app") {
			t.Errorf("%s ran in %q", c, c.Dir)
		}
	}
	seed := readFile(t, filepath.Join(dir, "demo-app", "package.json"))
	for _, want := range []string{
		`"name": "demo-app"`, `"private": true`, `"type": "module"`, `"build": "tsc"`,
		`"dev": "node --experimental-strip-types src/main.ts"`, `"start": "node dist/main.js"`, `"node": ">=22.6.0"`,
	} {
		if !strings.Contains(seed, want) {
			t.Errorf("seeded manifest lacks %s:\n%s", want, seed)
		}
	}
	for _, banned := range []string{`"main"`, `"types"`, `"files"`, `"test"`} {
		if strings.Contains(seed, banned) {
			t.Errorf("app manifest has %s:\n%s", banned, seed)
		}
	}
}

func TestInitAppFailureRemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	cmd := execx.Cmd{Name: "bun", Args: []string{"add", "--dev", "typescript", "@types/node"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1}}}}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo"}, fake); err == nil {
		t.Fatal("want the bun failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "demo")); !os.IsNotExist(err) {
		t.Errorf("app directory left behind: %v", err)
	}
}
