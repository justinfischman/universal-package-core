package register_test

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

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

// pythonNativeInterfaces are the native-check interfaces uv implements and the
// other Python managers must too, unless pythonInterfaceExceptions says why not.
var pythonNativeInterfaces = []optionalInterface{
	{"core.NativeDryRunner", implements[core.NativeDryRunner]},
	{"core.MemberLister", implements[core.MemberLister]},
}

// pythonInterfaceExceptions lists the native checks a Python manager truly
// lacks, with the reason (owner requirement D7.6).
var pythonInterfaceExceptions = map[string]map[string]string{
	"pip": {
		"core.NativeDryRunner": "pip publishes through `twine upload`, which has no dry-run flag (`twine check` validates only and does not exercise the upload)",
		"core.MemberLister":    "pip has no workspace concept: members are separate requirements files or editable installs",
	},
	"poetry": {
		"core.MemberLister": "poetry has no workspace feature; multi-project repos use path dependencies, which is not a member list",
	},
}

// TestPythonManagersHaveNativeCheckParity requires pip and poetry to implement
// each native-check interface uv implements, or to document why they cannot.
func TestPythonManagersHaveNativeCheckParity(t *testing.T) {
	regs := defaults(t)
	ref, ok := regs.PackageManagers.Get("uv")
	if !ok {
		t.Fatal("uv must be registered")
	}
	for _, id := range []string{"pip", "poetry"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, oi := range pythonNativeInterfaces {
			if oi.has(ref) && !oi.has(pm) && pythonInterfaceExceptions[id][oi.name] == "" {
				t.Errorf("%s lacks %s, which uv implements, with no documented exception", id, oi.name)
			}
		}
	}
}

// TestPythonInterfaceExceptionsStayAccurate fails when an exception is listed
// for an interface the manager now implements, so stale reasons are removed.
func TestPythonInterfaceExceptionsStayAccurate(t *testing.T) {
	regs := defaults(t)
	for id, byName := range pythonInterfaceExceptions {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, oi := range pythonNativeInterfaces {
			if byName[oi.name] != "" && oi.has(pm) {
				t.Errorf("%s implements %s but still lists an exception for it", id, oi.name)
			}
		}
	}
}
