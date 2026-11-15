package cli

import (
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// cobra's built-in completion command must stay enabled and produce a
// non-empty, shell-specific script for every supported shell.
func TestCompletionScriptsPerShell(t *testing.T) {
	markers := map[string]string{
		"bash":       "complete -o",
		"zsh":        "#compdef upc",
		"fish":       "complete -c upc",
		"powershell": "Register-ArgumentCompleter",
	}
	for shell, marker := range markers {
		t.Run(shell, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run("completion", shell); code != core.ExitOK {
				t.Fatalf("exit = %d, stderr = %q", code, h.stderr.String())
			}
			out := h.stdout.String()
			if strings.TrimSpace(out) == "" {
				t.Fatal("empty completion script")
			}
			if !strings.Contains(out, marker) {
				t.Errorf("script lacks marker %q", marker)
			}
			if !strings.Contains(out, "upc") {
				t.Error("script does not mention upc")
			}
		})
	}
}

func TestCompletionListedInHelp(t *testing.T) {
	h := newHarness(t)
	if code := h.run("--help"); code != core.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(h.stdout.String(), "completion") {
		t.Errorf("help does not list completion:\n%s", h.stdout.String())
	}
}
