package poetry

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
	fsys := groupFS("[tool.poetry]\nname = \"x\"\n[tool.poetry.group.dev.dependencies]\npytest = \"*\"\n[tool.poetry.group.docs]\noptional = true\n")
	if got := (&Poetry{}).DiagnoseGroups(fsys, []string{"main", "dev", "docs"}); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestDiagnoseGroupsLegacyDevDependenciesSatisfyDev(t *testing.T) {
	fsys := groupFS("[tool.poetry]\nname = \"x\"\n[tool.poetry.dev-dependencies]\npytest = \"*\"\n")
	if got := (&Poetry{}).DiagnoseGroups(fsys, []string{"dev"}); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestDiagnoseGroupsMissingGroupWarns(t *testing.T) {
	got := (&Poetry{}).DiagnoseGroups(groupFS("[tool.poetry]\nname = \"x\"\n"), []string{"docs"})
	if len(got) != 1 || got[0].Severity != core.SeverityWarn ||
		!strings.Contains(got[0].Message, `"docs" is not declared in [tool.poetry.group.docs]`) ||
		!strings.Contains(got[0].Message, "upc add <package> --group docs") {
		t.Errorf("findings = %v", got)
	}
}

func TestDiagnoseGroupsUnreadableProjectWarns(t *testing.T) {
	for name, fsys := range map[string]fstest.MapFS{"missing": {}, "invalid": groupFS("[tool")} {
		got := (&Poetry{}).DiagnoseGroups(fsys, []string{"dev"})
		if len(got) != 1 || got[0].Severity != core.SeverityWarn || !strings.Contains(got[0].Message, "cannot check") {
			t.Errorf("%s: findings = %v", name, got)
		}
	}
}
