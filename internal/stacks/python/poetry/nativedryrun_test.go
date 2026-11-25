package poetry

import (
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	p := newPoetry(t)
	if got, ok := p.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := p.NativeDryRunArgs(core.CapBuild); ok {
		t.Error("build must have no native dry run")
	}
}

func TestPublishWithNativeArgsRunsPoetryPublishDryRun(t *testing.T) {
	fake := &execx.FakeRunner{}
	args, _ := newPoetry(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"poetry publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}
