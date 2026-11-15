package cli

import (
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// requiredArgs gives each command the positionals it cannot run without.
func requiredArgs(spec capSpec) []string {
	switch spec.cap {
	case core.CapAdd, core.CapRemove:
		return []string{"pkg"}
	case core.CapRun:
		return []string{"script"}
	}
	return nil
}

// Every capability command, including up, down and publish, must carry the
// shared -w/--member flag into the request without per-command code.
func TestEveryCapabilityCommandHonorsMember(t *testing.T) {
	type row struct {
		verb string
		cap  core.Capability
		args []string
	}
	var rows []row
	for _, spec := range capabilityTable {
		rows = append(rows, row{strings.Fields(spec.use)[0], spec.cap, requiredArgs(spec)})
	}
	rows = append(rows, row{"publish", core.CapPublish, []string{"--yes"}})

	for _, r := range rows {
		for _, flag := range []string{"-w", "--member"} {
			t.Run(r.verb+" "+flag, func(t *testing.T) {
				h := newHarness(t)
				argv := append([]string{r.verb, flag, "api"}, r.args...)
				if code := h.run(argv...); code != core.ExitOK {
					t.Fatalf("exit %d: %s", code, h.stderr)
				}
				pm := h.pm(t, "alpha-one")
				var got *core.Request
				for i, c := range pm.Calls {
					if c == r.cap {
						got = &pm.Requests[i]
					}
				}
				if got == nil {
					t.Fatalf("capability %q not routed; calls = %v", r.cap, pm.Calls)
				}
				if got.Member != "api" {
					t.Errorf("Member = %q, want %q", got.Member, "api")
				}
			})
		}
	}
}
