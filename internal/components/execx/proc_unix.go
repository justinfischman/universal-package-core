//go:build !windows

package execx

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// startedAt reads the process start time with ps (-o lstart works on Linux and
// macOS); "" when ps is missing or the process is gone.
func startedAt(pid int) string {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(string(out)), " ")
}

func errorsIsNotFound(err error) bool { return errors.Is(err, exec.ErrNotFound) }

// detach puts the child in its own session so it is also its own process
// group: a terminal Ctrl-C does not reach it, and Stop can signal the group.
func detach(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func terminate(pid int) error { return signalGroup(pid, syscall.SIGTERM) }

func kill(pid int) error { return signalGroup(pid, syscall.SIGKILL) }

// signalGroup signals the whole process group, falling back to the process.
func signalGroup(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pid, sig); err == nil {
		return nil
	}
	return syscall.Kill(pid, sig)
}
