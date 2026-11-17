package execx

import (
	"errors"
	"os/exec"
	"strconv"

	"github.com/shirou/gopsutil/v4/process"
)

func errorsIsNotFound(err error) bool { return errors.Is(err, exec.ErrNotFound) }

func alive(pid int) bool {
	ok, err := process.PidExists(int32(pid))
	return err == nil && ok
}

// startedAt returns the process creation time (milliseconds since the epoch),
// "" when the process is gone or the platform refuses to say.
func startedAt(pid int) string {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return ""
	}
	ms, err := p.CreateTime()
	if err != nil {
		return ""
	}
	return strconv.FormatInt(ms, 10)
}

// procTree is a process and all of its descendants, captured before any of
// them is signalled (a child reparents once its parent exits).
type procTree []*process.Process

func treeOf(pid int) procTree {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	tree := procTree{root}
	for i := 0; i < len(tree); i++ {
		kids, err := tree[i].Children()
		if err != nil {
			continue // no children, or already gone
		}
		tree = append(tree, kids...)
	}
	return tree
}

// anyAlive reports whether any member of the tree is still running.
func (t procTree) anyAlive() bool {
	for _, p := range t {
		if alive(int(p.Pid)) {
			return true
		}
	}
	return false
}

// terminate asks every member to exit (SIGTERM; TerminateProcess on Windows).
func (t procTree) terminate() error {
	return t.each((*process.Process).Terminate)
}

// kill ends every member immediately.
func (t procTree) kill() error {
	return t.each((*process.Process).Kill)
}

func (t procTree) each(fn func(*process.Process) error) error {
	var errs []error
	for _, p := range t {
		if alive(int(p.Pid)) {
			errs = append(errs, fn(p))
		}
	}
	return errors.Join(errs...)
}
