package register_test

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Phase 6.3: every registered manager declares up, down and publish, and none
// of them is left missing or Unsupported (the five managers all fill them).
func TestEveryManagerDeclaresLifecycleCapabilities(t *testing.T) {
	managers := defaults(t).PackageManagers.List()
	if len(managers) == 0 {
		t.Fatal("no package managers registered")
	}
	for _, pm := range managers {
		caps := pm.Capabilities()
		for _, c := range []core.Capability{core.CapUp, core.CapDown, core.CapPublish} {
			t.Run(pm.ID()+"/"+string(c), func(t *testing.T) {
				s, ok := caps[c]
				if !ok {
					t.Fatalf("%s does not declare %q", pm.ID(), c)
				}
				if err := s.Validate(); err != nil {
					t.Fatalf("invalid declaration: %v", err)
				}
				if s.Level == core.LevelUnsupported {
					t.Errorf("%s declares %q unsupported (reason %q); expected native or filled", pm.ID(), c, s.Reason)
				}
			})
		}
	}
}
