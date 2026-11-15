package drift

import (
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestFindings(t *testing.T) {
	if got := Findings("env", "lock", nil, nil); got != nil {
		t.Errorf("no drift = %v", got)
	}
	got := Findings("env", "lock", []string{"b", "a"}, []string{"c (1 installed, 2 in lock)"})
	if len(got) != 2 {
		t.Fatalf("findings = %+v", got)
	}
	if got[0].Message != "env drifted from lock: a, b not installed in env" || got[0].Fix != core.CapInstall || got[0].Severity != core.SeverityWarn {
		t.Errorf("missing = %+v", got[0])
	}
	if !strings.HasSuffix(got[1].Message, "at a different version than lock") {
		t.Errorf("stale = %+v", got[1])
	}
}
