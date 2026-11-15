package poetry

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

func TestInitAppRunsPoetryNewThenAddsScripts(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "demo")
	if err := os.MkdirAll(root, 0o755); err != nil { // what `poetry new` would create
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[tool.poetry]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"poetry new --src demo"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	body, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[tool.poetry]", "[tool.upc.scripts]", `dev = "python server.py"`, `start = "python server.py"`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("pyproject.toml lacks %s:\n%s", want, body)
		}
	}
}

func TestInitAppDryRunAndBadInput(t *testing.T) {
	dir := t.TempDir()
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: dir, ProjectType: "app", Name: "demo", DryRun: true}, fake); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("dry run created %v", entries)
	}
	for name, req := range map[string]core.Request{
		"no name":   {Dir: dir, ProjectType: "app"},
		"flag-like": {Dir: dir, ProjectType: "app", Name: "--x"},
	} {
		fake := &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %q", name, err, argv(fake))
		}
	}
}
