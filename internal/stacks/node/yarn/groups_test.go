package yarn

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

const groupMember = ".universal-package-core/groups/docs/package.json"

// groupRunner answers `yarn --version` with the given version.
func groupRunner(version string) *execx.FakeRunner {
	cmd := execx.Cmd{Name: "yarn", Args: []string{"--version"}}
	return &execx.FakeRunner{Responses: map[string]execx.Response{cmd.String(): {Output: execx.Output{Stdout: version + "\n"}}}}
}

func groupFiles() map[string]string {
	return map[string]string{"package.json": `{"name":"demo"}`, groupMember: `{"name":"upc-group-docs"}`}
}

func TestAddToCustomGroupUsesWorkspaceCommand(t *testing.T) {
	dir := fixture(t, projectFiles)
	fake := groupRunner("4.1.0")
	if _, err := call(t, core.CapAdd, core.Request{Dir: dir, Packages: []string{"mkdocs"}, Group: "docs"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"yarn workspace upc-group-docs add mkdocs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(groupMember))); err != nil {
		t.Errorf("member manifest not created: %v", err)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	if !strings.Contains(string(root), ".universal-package-core/groups/*") {
		t.Errorf("root manifest does not register the groups glob: %s", root)
	}
}

func TestRemoveFromCustomGroupUsesWorkspaceCommand(t *testing.T) {
	fake := groupRunner("1.22.22")
	if _, err := call(t, core.CapRemove, core.Request{Dir: fixture(t, groupFiles()), Packages: []string{"mkdocs"}, Group: "docs"}, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"yarn workspace upc-group-docs remove mkdocs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestInstallCustomGroupBerryFocusesMembers(t *testing.T) {
	fake := groupRunner("4.1.0")
	req := core.Request{Dir: fixture(t, groupFiles()), Groups: []string{"docs"}}
	if _, err := call(t, core.CapInstall, req, fake); err != nil {
		t.Fatal(err)
	}
	if got, want := argv(fake), []string{"yarn workspaces focus upc-group-docs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func TestInstallCustomGroupClassicIsFullInstall(t *testing.T) {
	fake := groupRunner("1.22.22")
	req := core.Request{Dir: fixture(t, groupFiles()), Groups: []string{"docs"}}
	res, err := call(t, core.CapInstall, req, fake)
	if err != nil {
		t.Fatal(err)
	}
	if got := argv(fake); !reflect.DeepEqual(got, []string{"yarn install"}) {
		t.Errorf("argv = %q", got)
	}
	if !strings.Contains(res.Summary, "yarn 1 installs every group") {
		t.Errorf("summary hides the classic limit: %q", res.Summary)
	}
}

// workspaces focus is built in only from yarn 4; 2.x and 3.x fall back to a full
// install with a note.
func TestInstallCustomGroupBerryBeforeFourIsFullInstall(t *testing.T) {
	for _, version := range []string{"2.4.3", "3.6.4"} {
		fake := groupRunner(version)
		req := core.Request{Dir: fixture(t, groupFiles()), Groups: []string{"docs"}}
		res, err := call(t, core.CapInstall, req, fake)
		if err != nil {
			t.Fatal(err)
		}
		if got := argv(fake); !reflect.DeepEqual(got, []string{"yarn install"}) {
			t.Errorf("yarn %s argv = %q", version, got)
		}
		if !strings.Contains(res.Summary, "subset install needs yarn 4") {
			t.Errorf("yarn %s summary = %q", version, res.Summary)
		}
	}
}

func TestInstallBuiltinGroupIsFullInstallWithNote(t *testing.T) {
	fake := groupRunner("4.1.0")
	res, err := call(t, core.CapInstall, core.Request{Dir: fixture(t, projectFiles), Group: "dev"}, fake)
	if err != nil {
		t.Fatal(err)
	}
	if got := argv(fake); !reflect.DeepEqual(got, []string{"yarn install"}) || !strings.Contains(res.Summary, "built-in groups dev") {
		t.Errorf("argv = %q, summary = %q", got, res.Summary)
	}
}

func TestDiagnoseGroupsClassicNote(t *testing.T) {
	y := newYarn(t)
	files := groupFiles()
	files["package.json"] = `{"name":"demo","workspaces":[".universal-package-core/groups/*"]}`
	files["yarn.lock"] = "# yarn lockfile v1\n"
	got := y.DiagnoseGroups(os.DirFS(fixture(t, files)), []string{"docs"})
	if len(got) != 1 || got[0].Message != "yarn classic (1.x) has no subset install: `install` always installs every group" {
		t.Errorf("findings = %+v", got)
	}
	files["yarn.lock"] = "__metadata:\n  version: 6\n"
	if got := y.DiagnoseGroups(os.DirFS(fixture(t, files)), []string{"docs"}); len(got) != 0 {
		t.Errorf("berry findings = %+v", got)
	}
}
