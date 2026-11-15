package execx

import (
	"context"
	"sync"
)

// Response is a scripted result for a FakeRunner.
type Response struct {
	Output Output
	Err    error
}

// FakeRunner records calls and returns scripted responses keyed by the
// command's String(). Unscripted commands succeed with empty output.
type FakeRunner struct {
	mu        sync.Mutex
	Calls     []Cmd
	Responses map[string]Response
	// Procs are the background processes started so far; StartErr and StopErr
	// script failures of Start and Stop.
	Procs    []*FakeProcess
	StartErr error
	StopErr  error
}

// Run implements Runner.
func (f *FakeRunner) Run(_ context.Context, cmd Cmd) (Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, cmd)
	if r, ok := f.Responses[cmd.String()]; ok {
		return r.Output, r.Err
	}
	return Output{}, nil
}

// FakeProcess is a scripted background process for a FakeRunner.
type FakeProcess struct {
	PID     int
	Cmd     Cmd
	LogPath string
	Running bool
}

var _ Controller = (*FakeRunner)(nil)

// Start implements Controller. It hands out increasing pids from 1000, writes
// nothing, and fails with StartErr when that is set.
func (f *FakeRunner) Start(_ context.Context, cmd Cmd, logPath string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, cmd)
	if f.StartErr != nil {
		return 0, f.StartErr
	}
	pid := 1000 + len(f.Procs)
	f.Procs = append(f.Procs, &FakeProcess{PID: pid, Cmd: cmd, LogPath: logPath, Running: true})
	return pid, nil
}

// Alive implements Controller.
func (f *FakeRunner) Alive(pid int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.Procs {
		if p.PID == pid {
			return p.Running
		}
	}
	return false
}

// Stop implements Controller.
func (f *FakeRunner) Stop(pid int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StopErr != nil {
		return f.StopErr
	}
	for _, p := range f.Procs {
		if p.PID == pid {
			p.Running = false
		}
	}
	return nil
}

// Crash marks a started process as having died on its own.
func (f *FakeRunner) Crash(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.Procs {
		if p.PID == pid {
			p.Running = false
		}
	}
}
