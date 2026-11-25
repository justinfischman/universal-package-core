package uv

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func groupFS(pyproject string) fstest.MapFS {
	return fstest.MapFS{"pyproject.toml": {Data: []byte(pyproject)}}
}

func TestDiagnoseGroupsDeclaredGroupsAreClean(t *testing.T) {
	fsys := groupFS("[project]\nname = \"x\"\n[dependency-groups]\ndev = [\"pytest\"]\nDocs_Site = [\"mkdocs\"]\n")
	if got := (&UV{}).DiagnoseGroups(fsys, []string{"dev", "docs-site"}); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestDiagnoseGroupsMissingGroupWarns(t *testing.T) {
	got := (&UV{}).DiagnoseGroups(groupFS("[project]\nname = \"x\"\n"), []string{"docs"})
	if len(got) != 1 || got[0].Severity != core.SeverityWarn ||
		!strings.Contains(got[0].Message, `"docs" is not declared in [dependency-groups]`) ||
		!strings.Contains(got[0].Message, "upc add <package> --group docs") {
		t.Errorf("findings = %v", got)
	}
}

func TestDiagnoseGroupsExtraOnlyWarns(t *testing.T) {
	got := (&UV{}).DiagnoseGroups(groupFS("[project]\nname = \"x\"\n[project.optional-dependencies]\ndocs = [\"mkdocs\"]\n"), []string{"docs"})
	if len(got) != 1 || !strings.Contains(got[0].Message, "only in [project.optional-dependencies]") {
		t.Errorf("findings = %v", got)
	}
}

func TestDiagnoseGroupsUnreadableProjectWarns(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{"missing": {}, "invalid": groupFS("[project")} {
		got := (&UV{}).DiagnoseGroups(fsys, []string{"dev"})
		if len(got) != 1 || got[0].Severity != core.SeverityWarn || !strings.Contains(got[0].Message, "cannot check") {
			t.Errorf("%s: findings = %v", name, got)
		}
	}
}

func TestDiagnoseGroupsNoGroupsReportsNothing(t *testing.T) {
	if got := (&UV{}).DiagnoseGroups(fstest.MapFS{}, nil); got != nil {
		t.Errorf("findings = %v, want nil", got)
	}
}
