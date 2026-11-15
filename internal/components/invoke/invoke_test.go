package invoke

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInvocationNeedsARunner(t *testing.T) {
	if _, err := Begin(context.Background(), core.Request{}, "x"); err == nil {
		t.Error("missing runner must be an error")
	}
	fake := &execx.FakeRunner{}
	inv, err := Begin(execx.WithRunner(context.Background(), fake), core.Request{Dir: "d", DryRun: true}, "tool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inv.Run(false, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := inv.RunProgram(true, "other", "b"); err != nil {
		t.Fatal(err)
	}
	res := inv.Result("done", "do it", nil)
	if !reflect.DeepEqual(res.Commands, []string{"tool a", "other b"}) || res.Summary != "dry run, nothing changed: would do it" {
		t.Errorf("result = %+v", res)
	}
	if fake.Calls[0].Dir != "d" || fake.Calls[0].Stream || !fake.Calls[1].Stream {
		t.Errorf("calls = %+v", fake.Calls)
	}
}

func TestAlternativesReplaceAMissingProgram(t *testing.T) {
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		"first a": {Err: &core.ToolMissingError{Tool: "first"}},
	}}
	inv, err := Begin(execx.WithRunner(context.Background(), fake), core.Request{}, "first")
	if err != nil {
		t.Fatal(err)
	}
	inv.WithAlternatives(func(name string) []string { return []string{"second"} })
	if _, err := inv.Run(false, "a"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inv.Commands, []string{"second a"}) {
		t.Errorf("commands = %v, want only the one that ran", inv.Commands)
	}
}

func TestAlternativesAllMissingReportTheFirstProgram(t *testing.T) {
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		"first a":  {Err: &core.ToolMissingError{Tool: "first"}},
		"second a": {Err: &core.ToolMissingError{Tool: "second"}},
	}}
	inv, _ := Begin(execx.WithRunner(context.Background(), fake), core.Request{}, "first")
	inv.WithAlternatives(func(string) []string { return []string{"second"} })
	_, err := inv.Run(false, "a")
	var missing *core.ToolMissingError
	if !errors.As(err, &missing) || missing.Tool != "first" {
		t.Errorf("err = %v, want tool missing for first", err)
	}
}
