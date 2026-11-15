package runstate

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
)

func TestSameDirectoryDifferentAppsDoNotClash(t *testing.T) {
	dir := t.TempDir()
	fake := &execx.FakeRunner{}
	api := State{Dir: dir, App: "api", Ctl: fake}
	web := State{Dir: dir, App: "web", Ctl: fake}
	if api.RunDir() == web.RunDir() {
		t.Fatalf("run directories clash: %s", api.RunDir())
	}
	if want := filepath.Join(dir, ".universal-package-core", "run", "api"); api.RunDir() != want {
		t.Errorf("RunDir = %q, want %q", api.RunDir(), want)
	}
	if _, err := api.Start(context.Background(), execx.Cmd{Name: "srv", Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if web.Running() {
		t.Error("web reports running after only api started")
	}
	if out, err := web.Stop(); err != nil || out != Absent {
		t.Errorf("web Stop = %v, %v; want absent", out, err)
	}
	if !api.Running() {
		t.Error("api stopped by stopping web")
	}
}
