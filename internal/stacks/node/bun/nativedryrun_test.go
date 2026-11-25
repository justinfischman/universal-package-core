package bun

import (
	"context"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	b := newBun(t)
	if got, ok := b.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := b.NativeDryRunArgs(core.CapTest); ok {
		t.Error("test must have no native dry run")
	}
}

func TestPublishWithNativeArgsRunsBunPublishDryRun(t *testing.T) {
	fake := &execx.FakeRunner{}
	args, _ := newBun(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"bun publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNativeMembersReadsPackageJSONWorkspaces(t *testing.T) {
	dir := fixture(t, map[string]string{"package.json": `{"name":"r","workspaces":["packages/*"]}`})
	got, err := newBun(t).NativeMembers(context.Background(), dir)
	if err != nil || !reflect.DeepEqual(got, []string{"packages/*"}) {
		t.Errorf("members = %q, %v", got, err)
	}
}
