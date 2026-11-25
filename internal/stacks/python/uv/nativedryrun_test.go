package uv

import (
	"context"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	u := newUV(t)
	if got, ok := u.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := u.NativeDryRunArgs(core.CapBuild); ok {
		t.Error("build must have no native dry run")
	}
}

func TestPublishWithNativeArgsRunsUvPublishDryRun(t *testing.T) {
	fake := &execx.FakeRunner{}
	args, _ := newUV(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"uv publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNativeMembersReadsUvWorkspaceTable(t *testing.T) {
	pyproject := "[project]\nname = \"root\"\n\n[tool.uv.workspace]\nmembers = [\"packages/*\", \"tools/cli\"]\nexclude = [\"packages/legacy\"]\n"
	got, err := newUV(t).NativeMembers(context.Background(), fixture(t, map[string]string{"pyproject.toml": pyproject}))
	if want := []string{"packages/*", "tools/cli", "!packages/legacy"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("members = %q, %v; want %q", got, err, want)
	}
}

func TestNativeMembersWithoutWorkspaceTableIsEmpty(t *testing.T) {
	got, err := newUV(t).NativeMembers(context.Background(), fixture(t, projectFiles))
	if err != nil || len(got) != 0 {
		t.Errorf("members = %q, %v; want none", got, err)
	}
}

func TestNativeMembersErrors(t *testing.T) {
	if _, err := newUV(t).NativeMembers(context.Background(), t.TempDir()); err == nil {
		t.Error("want an error without pyproject.toml")
	}
	if _, err := newUV(t).NativeMembers(context.Background(), fixture(t, map[string]string{"pyproject.toml": "[tool.uv\n"})); err == nil {
		t.Error("want a parse error for malformed TOML")
	}
}
