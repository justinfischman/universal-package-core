//go:build windows

package execx

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

const processQueryLimitedInformation = 0x1000

// startedAt returns the process creation time (GetProcessTimes), "" on failure.
func startedAt(pid int) string {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(h)
	var creation, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return ""
	}
	return strconv.FormatInt(creation.Nanoseconds(), 10)
}

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

// kill ends the process and its children: taskkill /T walks the process tree
// (p.Kill alone would leave grandchildren running). It falls back to Kill for
// the process itself if taskkill is unavailable.
func kill(pid int) error {
	if err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run(); err == nil {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer p.Release()
	return p.Kill()
}
