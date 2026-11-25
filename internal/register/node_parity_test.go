package register_test

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// nodeParityExceptions lists capabilities a Node manager may support at a
// different level than npm, with the reason. It is empty: pnpm and bun match
// npm and yarn on every capability (owner requirement D7.6).
var nodeParityExceptions = map[string]map[core.Capability]string{}

// TestNodeManagersHaveCapabilityParity requires every Node manager to declare
// each capability at the same level as npm (native, filled or unsupported),
// unless nodeParityExceptions documents why not.
func TestNodeManagersHaveCapabilityParity(t *testing.T) {
	regs := defaults(t)
	ref, ok := regs.PackageManagers.Get("npm")
	if !ok {
		t.Fatal("npm must be registered")
	}
	for _, id := range []string{"yarn", "pnpm", "bun"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, c := range core.AllCapabilities {
			want, got := ref.Capabilities()[c].Level, pm.Capabilities()[c].Level
			if want == got {
				continue
			}
			if reason := nodeParityExceptions[id][c]; reason == "" {
				t.Errorf("%s declares %s as %v, npm as %v, with no documented exception", id, c, got, want)
			}
		}
	}
}
