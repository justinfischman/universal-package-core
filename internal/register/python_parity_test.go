package register_test

import "testing"

// pythonGroupDiagnoserExceptions lists Python managers that may lack the group
// diagnoser, with the reason. It is empty: uv, poetry and pip all check their
// group model (owner requirement D7.6).
var pythonGroupDiagnoserExceptions = map[string]string{}

// TestPythonManagersHaveGroupDiagnoser requires every Python manager to
// implement DiagnoseGroups, as the Node managers do, so `doctor` checks
// dependency groups for all of them. The probe is a type assertion, so a
// removed method fails here.
func TestPythonManagersHaveGroupDiagnoser(t *testing.T) {
	regs := defaults(t)
	for _, id := range []string{"uv", "poetry", "pip"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		if !implements[groupDiagnoser](pm) && pythonGroupDiagnoserExceptions[id] == "" {
			t.Errorf("%s lacks DiagnoseGroups with no documented exception", id)
		}
	}
}
