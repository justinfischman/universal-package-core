//go:build windows

package execx

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

func errorsIsNotFound(err error) bool { return errors.Is(err, exec.ErrNotFound) }

// detach starts the child outside the console's process group.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}

// alive: on Windows FindProcess opens a handle and fails when no such process
// exists; Signal(0) is not supported there.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}

// terminate has no graceful equivalent for a detached process; Stop falls
// through to kill after the grace period, so terminate kills directly.
func terminate(pid int) error { return kill(pid) }

func kill(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer p.Release()
	return p.Kill()
}
