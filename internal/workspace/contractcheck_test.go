package workspace

import (
	"os"
	"strings"
	"testing"
)

func findingOf(w *Workspace, kind FindingKind) (Finding, int) {
	n := 0
	var found Finding
	for _, f := range w.Findings() {
		if f.Kind == kind {
			found = f
			n++
		}
	}
	return found, n
}

func TestFindingContractLockMissing(t *testing.T) {
	locks, _ := lockFixture(t, specV1)
	f, n := findingOf(locks.workspace, ContractLockMissing)
	if n != 1 || !strings.Contains(f.Message, `"api"`) {
		t.Fatalf("missing findings = %d %v", n, f)
	}
	if _, n := findingOf(locks.workspace, ContractLockChanged); n != 0 {
		t.Errorf("changed findings = %d, want 0", n)
	}
}

func TestFindingContractLockChangedAndClean(t *testing.T) {
	locks, file := lockFixture(t, specV1)
	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	if _, n := findingOf(locks.workspace, ContractLockMissing); n != 0 {
		t.Errorf("missing findings after update = %d", n)
	}
	if _, n := findingOf(locks.workspace, ContractLockChanged); n != 0 {
		t.Errorf("changed findings after update = %d", n)
	}
	if err := os.WriteFile(file, []byte(specV2), 0o644); err != nil {
		t.Fatal(err)
	}
	f, n := findingOf(locks.workspace, ContractLockChanged)
	if n != 1 || !strings.Contains(f.Message, "POST /pets") {
		t.Fatalf("changed findings = %d %v", n, f)
	}
}

func TestFindingContractUnreadableCountsAsChanged(t *testing.T) {
	locks, file := lockFixture(t, specV1)
	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	f, n := findingOf(locks.workspace, ContractLockChanged)
	if n != 1 || !strings.Contains(f.Message, "cannot be compared") {
		t.Fatalf("findings = %d %v", n, f)
	}
}

func TestCheckAllSkipsAppsWithoutContract(t *testing.T) {
	locks, _ := lockFixture(t, specV1)
	got := locks.CheckAll()
	if len(got) != 1 || got[0].App != "api" || !got[0].Failing() || got[0].Status.State != ContractMissing {
		t.Fatalf("CheckAll = %+v", got)
	}
	if err := locks.Update("api"); err != nil {
		t.Fatal(err)
	}
	if got := locks.CheckAll(); got[0].Failing() {
		t.Fatalf("after update: %+v", got)
	}
}
