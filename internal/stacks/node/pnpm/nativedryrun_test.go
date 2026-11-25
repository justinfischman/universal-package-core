package pnpm

import (
	"context"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	p := newPnpm(t)
	if got, ok := p.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := p.NativeDryRunArgs(core.CapLint); ok {
		t.Error("lint must have no native dry run")
	}
}

func TestPublishWithNativeArgsRunsPnpmPublishDryRun(t *testing.T) {
	fake := &execx.FakeRunner{}
	args, _ := newPnpm(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"pnpm publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNativeMembersReadsPnpmWorkspaceYAML(t *testing.T) {
	files := map[string]string{
		"package.json":        `{"name":"r","workspaces":["ignored/*"]}`,
		"pnpm-workspace.yaml": "packages:\n  - 'apps/*'\n  - \"!apps/old\"\n  - libs/core\n",
	}
	got, err := newPnpm(t).NativeMembers(context.Background(), fixture(t, files))
	if want := []string{"apps/*", "!apps/old", "libs/core"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("members = %q, %v; want %q", got, err, want)
	}
}

func TestNativeMembersWithoutWorkspaceFileIsEmpty(t *testing.T) {
	got, err := newPnpm(t).NativeMembers(context.Background(), fixture(t, projectFiles))
	if err != nil || len(got) != 0 {
		t.Errorf("members = %q, %v; want none", got, err)
	}
}

func TestNativeMembersRejectsMalformedYAML(t *testing.T) {
	files := map[string]string{"pnpm-workspace.yaml": "packages: [unclosed\n"}
	if _, err := newPnpm(t).NativeMembers(context.Background(), fixture(t, files)); err == nil {
		t.Error("want a parse error")
	}
}
