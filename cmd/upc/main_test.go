package main

import (
	"os"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// runWith runs the real main entry point (real registry and dependencies) with
// args in an empty directory and an isolated UPC_HOME, so no project or user
// config is read.
func runWith(t *testing.T, args ...string) int {
	t.Helper()
	t.Setenv("UPC_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	os.Args = append([]string{"upc"}, args...)
	return run()
}

func TestSmokeExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"version", []string{"version"}, core.ExitOK},
		{"help", []string{"--help"}, core.ExitOK},
		{"unknown command", []string{"definitely-not-a-command"}, core.ExitUsage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runWith(t, c.args...); got != c.want {
				t.Errorf("upc %v exited %d, want %d", c.args, got, c.want)
			}
		})
	}
}
