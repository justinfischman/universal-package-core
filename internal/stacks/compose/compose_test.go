package compose_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/compose"
)

func dirWith(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDetect(t *testing.T) {
	p := compose.New(&execx.FakeRunner{})
	for _, name := range []string{"docker-compose.yml", "compose.yaml"} {
		if !p.Detect(dirWith(t, name)) {
			t.Errorf("%s not detected", name)
		}
	}
	if p.Detect(t.TempDir()) {
		t.Error("empty dir detected")
	}
}

func TestUpAndDownArgv(t *testing.T) {
	dir := dirWith(t, "compose.yaml")
	f := &execx.FakeRunner{}
	p := compose.New(f)
	target := core.UpTarget{Dir: dir, App: "Web App"}
	if err := p.Up(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	if err := p.Down(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"docker compose -p web-app -f compose.yaml up -d",
		"docker compose -p web-app -f compose.yaml down",
	}
	for i, w := range want {
		if got := f.Calls[i]; got.String() != w || got.Dir != dir {
			t.Errorf("call %d: %q dir %q, want %q", i, got.String(), got.Dir, w)
		}
	}
}

func TestProjectNameDefaultsToDirectory(t *testing.T) {
	if got := compose.ProjectName(core.UpTarget{Dir: "/x/My.Service"}); got != "my-service" {
		t.Errorf("got %q", got)
	}
}

func TestAbsentFileIsAnErrorAndRunsNothing(t *testing.T) {
	f := &execx.FakeRunner{}
	if err := compose.New(f).Up(context.Background(), core.UpTarget{Dir: t.TempDir()}); err == nil {
		t.Error("want error")
	}
	if len(f.Calls) != 0 {
		t.Errorf("calls %d", len(f.Calls))
	}
}
