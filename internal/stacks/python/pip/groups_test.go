package pip

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestDiagnoseGroupsPresentFilesAreClean(t *testing.T) {
	fsys := fstest.MapFS{"requirements-dev.txt": {Data: []byte("pytest\n")}, "requirements-docs.txt": {}}
	if got := (&PIP{}).DiagnoseGroups(fsys, []string{"dev", "docs"}); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestDiagnoseGroupsMissingFileWarns(t *testing.T) {
	got := (&PIP{}).DiagnoseGroups(fstest.MapFS{"requirements.txt": {}}, []string{"docs"})
	if len(got) != 1 || got[0].Severity != core.SeverityWarn ||
		!strings.Contains(got[0].Message, `"docs" has no requirements-docs.txt`) ||
		!strings.Contains(got[0].Message, "upc add <package> --group docs") {
		t.Errorf("findings = %v", got)
	}
}

func TestDiagnoseGroupsInvalidNameWarns(t *testing.T) {
	got := (&PIP{}).DiagnoseGroups(fstest.MapFS{}, []string{"../evil"})
	if len(got) != 1 || got[0].Severity != core.SeverityWarn || !strings.Contains(got[0].Message, "not a valid group") {
		t.Errorf("findings = %v", got)
	}
}

func TestDiagnoseGroupsNoGroupsReportsNothing(t *testing.T) {
	if got := (&PIP{}).DiagnoseGroups(fstest.MapFS{}, nil); got != nil {
		t.Errorf("findings = %v, want nil", got)
	}
}
