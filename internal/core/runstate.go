package core

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// RunState records execution state for up/down lifecycle.
type RunState struct {
	Dir string
}

func (r RunState) PidFile(app string) string {
	if app == "" {
		app = "default"
	}
	return filepath.Join(r.Dir, ".universal-package-core", "run", app+".pid")
}

func (r RunState) WritePid(app string, pid int) error {
	path := r.PidFile(app)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}

func (r RunState) ReadPid(app string) (int, error) {
	data, err := os.ReadFile(r.PidFile(app))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func (r RunState) RemovePid(app string) error {
	path := r.PidFile(app)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(path)
}

func (r RunState) IsRunning(app string) bool {
	pid, err := r.ReadPid(app)
	if err != nil {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
