//go:build !windows

package execx

import (
	"os/exec"
	"syscall"
)

// detach puts the child in its own session, so a terminal Ctrl-C does not
// reach it.
func detach(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
