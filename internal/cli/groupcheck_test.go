package cli

import (
	"io/fs"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// groupStubPM records the groups doctor asks it to check.
type groupStubPM struct {
	core.PackageManager
	asked [][]string
}

func (g *groupStubPM) DiagnoseGroups(_ fs.FS, groups []string) []core.Finding {
	g.asked = append(g.asked, groups)
	return []core.Finding{{Severity: core.SeverityWarn, Message: "drift in " + groups[0]}}
}

func groupResolved() config.Resolved {
	return config.Resolved{Config: config.Config{Environments: map[string]config.Environment{
		"zeta":  {Groups: []string{"docs"}},
		"alpha": {Groups: []string{"test", "e2e"}},
		"plain": {File: ".env"},
	}}}
}

func TestGroupFindingsPerEnvironmentInNameOrder(t *testing.T) {
	pm := &groupStubPM{}
	got := groupFindings(pm, emptyFS{}, groupResolved())
	if want := [][]string{{"test", "e2e"}, {"docs"}}; !reflect.DeepEqual(pm.asked, want) {
		t.Errorf("asked = %v, want %v", pm.asked, want)
	}
	if len(got) != 2 || got[0].Message != `environment "alpha": drift in test` || got[1].Message != `environment "zeta": drift in docs` {
		t.Errorf("findings = %+v", got)
	}
}

func TestGroupFindingsSkipsManagersThatCannotCheck(t *testing.T) {
	var pm core.PackageManager
	if got := groupFindings(pm, emptyFS{}, groupResolved()); got != nil {
		t.Errorf("findings = %+v, want none", got)
	}
}
