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
