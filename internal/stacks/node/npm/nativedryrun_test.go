package npm

import (
	"context"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNativeDryRunArgsArePublishOnly(t *testing.T) {
	n := newNPM(t)
	if got, ok := n.NativeDryRunArgs(core.CapPublish); !ok || !reflect.DeepEqual(got, []string{"--dry-run"}) {
		t.Errorf("publish = %q, %v", got, ok)
	}
	if _, ok := n.NativeDryRunArgs(core.CapBuild); ok {
		t.Error("build must have no native dry run")
	}
}

func TestPublishWithNativeArgsRunsNpmPublishDryRun(t *testing.T) {
	fake := &execx.FakeRunner{}
	args, _ := newNPM(t).NativeDryRunArgs(core.CapPublish)
	if _, err := call(t, core.CapPublish, core.Request{Dir: fixture(t, projectFiles), Args: args}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"npm publish --dry-run"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestNativeMembersReadsPackageJSONWorkspaces(t *testing.T) {
	cases := map[string]struct {
		manifest string
		want     []string
	}{
		"array":   {`{"name":"r","workspaces":["apps/*","!apps/old"]}`, []string{"apps/*", "!apps/old"}},
		"object":  {`{"name":"r","workspaces":{"packages":["libs/a"]}}`, []string{"libs/a"}},
		"missing": {`{"name":"r"}`, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := fixture(t, map[string]string{"package.json": tc.manifest})
			got, err := newNPM(t).NativeMembers(context.Background(), dir)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("members = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestNativeMembersErrorsWithoutManifest(t *testing.T) {
	if _, err := newNPM(t).NativeMembers(context.Background(), t.TempDir()); err == nil {
		t.Error("want an error for a directory without package.json")
	}
}
