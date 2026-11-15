// Package drift builds the findings for a project whose installed environment
// no longer matches what its lock or requirements file records.
package drift

import (
	"fmt"
	"sort"
	"strings"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// Findings reports packages missing from envDir and packages installed at a
// different version than source records. Each group becomes one warning whose
// fix is install; empty groups report nothing. The names are sorted.
func Findings(envDir, source string, missing, stale []string) []core.Finding {
	var out []core.Finding
	for _, group := range []struct {
		names []string
		text  string
	}{
		{missing, "not installed in " + envDir},
		{stale, "at a different version than " + source},
	} {
		if len(group.names) == 0 {
			continue
		}
		sort.Strings(group.names)
		out = append(out, core.Finding{
			Severity: core.SeverityWarn,
			Message:  fmt.Sprintf("%s drifted from %s: %s %s", envDir, source, strings.Join(group.names, ", "), group.text),
			Fix:      core.CapInstall,
		})
	}
	return out
}
