package yarn

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	y := newYarn(t)
	if got, ok := y.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := y.NativeDryRunArgs(core.CapInstall); ok {
		t.Error("install must have no native dry run")
	}
}

func TestBerryPublishWithNativeArgsRunsYarnNpmPublishDryRun(t *testing.T) {
	fake := groupRunner("4.1.0")
	args, _ := newYarn(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"yarn npm publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestClassicPublishRefusesDryRunInsteadOfPublishing(t *testing.T) {
	fake := groupRunner("1.22.22")
	_, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: []string{"--dry-run"}}, fake)
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	if got := argv(fake); len(got) != 0 {
		t.Errorf("ran %q; classic must not publish", got)
	}
}

func TestNativeMembersReadsPackageJSONWorkspaces(t *testing.T) {
	dir := fixture(t, map[string]string{"package.json": `{"name":"r","workspaces":{"packages":["packages/*"]}}`})
	got, err := newYarn(t).NativeMembers(context.Background(), dir)
	if err != nil || !reflect.DeepEqual(got, []string{"packages/*"}) {
		t.Errorf("members = %q, %v", got, err)
	}
}
