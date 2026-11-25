package bun

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

const groupDir = ".universal-package-core/groups/docs"

func TestAddToCustomGroupCreatesMemberAndUsesFilterFlag(t *testing.T) {
	dir := fixture(t, projectFiles)
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"mkdocs"}, Group: "docs"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"bun add --filter upc-group-docs mkdocs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(groupDir), "package.json")); err != nil {
		t.Errorf("member manifest not created: %v", err)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	if !strings.Contains(string(root), ".universal-package-core/groups/*") {
		t.Errorf("root manifest does not register the groups glob: %s", root)
	}
}

func TestAddToCustomGroupDryRunWritesNothing(t *testing.T) {
	dir := fixture(t, projectFiles)
	if _, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "docs", DryRun: true}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".universal-package-core")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dry run created group files: %v", err)
	}
}

func TestInstallCustomGroupsUsesFilterFlagPerGroup(t *testing.T) {
	dir := fixture(t, map[string]string{
		"package.json":             `{"name":"demo"}`,
		groupDir + "/package.json": `{"name":"upc-group-docs"}`,
		".universal-package-core/groups/e2e/package.json": `{"name":"upc-group-e2e"}`,
	})
	fake := &execx.FakeRunner{}
	req := core.Request{Dir: dir, Group: "dev", Groups: []string{"docs", "e2e"}}
	if _, err := call(t, core.CapInstall, req, fake); err != nil {
		t.Fatal(err)
	}
	want := []string{"bun install --filter upc-group-docs --filter upc-group-e2e"}
	if got := argv(fake); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestInstallBuiltinGroupsOnlyIsFullInstall(t *testing.T) {
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInstall, core.Request{Dir: fixture(t, projectFiles), Group: "dev"}, fake); err != nil {
		t.Fatal(err)
	}
	if got := argv(fake); !reflect.DeepEqual(got, []string{"bun install"}) {
		t.Errorf("argv = %q", got)
	}
}

func TestRemoveFromCustomGroup(t *testing.T) {
	dir := fixture(t, map[string]string{"package.json": `{"name":"demo"}`, groupDir + "/package.json": `{}`})
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapRemove, core.Request{Dir: dir, Packages: []string{"mkdocs"}, Group: "docs"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"bun remove --filter upc-group-docs mkdocs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestInstallGroupWithInvalidNameRunsNothing(t *testing.T) {
	fake := &execx.FakeRunner{}
	_, err := call(t, core.CapInstall, core.Request{Dir: fixture(t, projectFiles), Groups: []string{"../x"}}, fake)
	if !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
		t.Errorf("err = %v, calls = %v", err, fake.Calls)
	}
}

func TestDiagnoseGroupsReportsMissingMember(t *testing.T) {
	got := newBun(t).DiagnoseGroups(os.DirFS(fixture(t, projectFiles)), []string{"docs"})
	if len(got) != 2 {
		t.Errorf("findings = %+v, want 2", got)
	}
}
