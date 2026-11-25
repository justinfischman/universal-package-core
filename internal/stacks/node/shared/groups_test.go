package shared

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func groupsProject(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestValidateGroupName(t *testing.T) {
	for _, ok := range []string{"docs", "test-1", "a_b"} {
		if err := ValidateGroupName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "..", "-x", "A", "a/b", "a b", "a.b"} {
		if err := ValidateGroupName(bad); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q err = %v, want ErrInvalidRequest", bad, err)
		}
	}
}

func TestCustomGroupsSkipsBuiltins(t *testing.T) {
	got, err := CustomGroups([]string{"dev", "docs", "peer", "e2e"})
	if err != nil || strings.Join(got, ",") != "docs,e2e" {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestEnsureMemberKeepsManifestOrderAndRegistersGlob(t *testing.T) {
	dir := groupsProject(t, "{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\",\n  \"scripts\": {\"a\": \"b\"}\n}\n")
	for range 2 { // the second call must change nothing
		if err := EnsureMember(fsx.OS{}, dir, "docs"); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	want := "{\n  \"name\": \"demo\",\n  \"version\": \"1.0.0\",\n  \"scripts\": {\n    \"a\": \"b\"\n  },\n  \"workspaces\": [\n    \".universal-package-core/groups/*\"\n  ]\n}\n"
	if string(data) != want {
		t.Errorf("package.json =\n%s\nwant\n%s", data, want)
	}
	member, err := os.ReadFile(filepath.Join(dir, ".universal-package-core", "groups", "docs", "package.json"))
	if err != nil || !strings.Contains(string(member), `"name": "upc-group-docs"`) || !strings.Contains(string(member), `"private": true`) {
		t.Errorf("member = %s, %v", member, err)
	}
}

func TestEnsureMemberAppendsToExistingWorkspaces(t *testing.T) {
	cases := map[string]string{
		"list":   `{"name":"d","workspaces":["packages/*"]}`,
		"object": `{"name":"d","workspaces":{"packages":["packages/*"],"nohoist":["x"]}}`,
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			dir := groupsProject(t, manifest)
			if err := EnsureMember(fsx.OS{}, dir, "docs"); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
			globs, err := Workspaces(data)
			if err != nil || strings.Join(globs, ",") != "packages/*,"+GroupsGlob {
				t.Errorf("workspaces = %v, %v", globs, err)
			}
			if name == "object" && !strings.Contains(string(data), "nohoist") {
				t.Errorf("object form lost nohoist: %s", data)
			}
		})
	}
}

func TestEnsureMemberRejectsBadInput(t *testing.T) {
	dir := groupsProject(t, `{"name":"d"}`)
	if err := EnsureMember(fsx.OS{}, dir, "../evil"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("bad name err = %v", err)
	}
	broken := groupsProject(t, `{"name":`)
	if err := EnsureMember(fsx.OS{}, broken, "docs"); err == nil {
		t.Error("malformed package.json accepted")
	}
}

func TestGroupFindings(t *testing.T) {
	project := fstest.MapFS{"package.json": {Data: []byte(`{"name":"d"}`)}}
	got := GroupFindings(project, []string{"dev", "docs"}, false)
	if len(got) != 2 || !strings.Contains(got[0].Message, `"docs" has no member`) || !strings.Contains(got[1].Message, "workspaces does not list") {
		t.Errorf("missing member and glob: %+v", got)
	}
	ready := fstest.MapFS{
		"package.json":                     {Data: []byte(`{"name":"d","workspaces":[".universal-package-core/groups/*"]}`)},
		GroupDir("docs") + "/package.json": {Data: []byte(`{}`)},
	}
	if got := GroupFindings(ready, []string{"docs"}, false); len(got) != 0 {
		t.Errorf("healthy project: %+v", got)
	}
	if got := GroupFindings(ready, []string{"docs"}, true); len(got) != 1 || got[0].Severity != core.SeverityInfo {
		t.Errorf("classic note: %+v", got)
	}
	if got := GroupFindings(project, []string{"dev"}, true); len(got) != 0 {
		t.Errorf("builtin only: %+v", got)
	}
	if got := GroupFindings(project, []string{"Bad Name"}, false); len(got) != 1 {
		t.Errorf("invalid name: %+v", got)
	}
	broken := fstest.MapFS{
		"package.json":                     {Data: []byte(`{"name":`)},
		GroupDir("docs") + "/package.json": {Data: []byte(`{}`)},
	}
	if got := GroupFindings(broken, []string{"docs"}, false); len(got) != 1 || !strings.Contains(got[0].Message, "cannot check the workspaces") {
		t.Errorf("malformed root manifest: %+v", got)
	}
}
