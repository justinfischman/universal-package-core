package shared

import (
	"context"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInterpreterAlternatives(t *testing.T) {
	if got := InterpreterAlternatives("python3"); !reflect.DeepEqual(got, []string{"python", "py"}) {
		t.Errorf("python3 -> %v", got)
	}
	if got := InterpreterAlternatives("py"); !reflect.DeepEqual(got, []string{"python3", "python"}) {
		t.Errorf("py -> %v", got)
	}
	if got := InterpreterAlternatives(".venv/bin/python"); got != nil {
		t.Errorf("a project interpreter has no alternatives, got %v", got)
	}
}

func run(t *testing.T, missing ...string) (*execx.FakeRunner, []string, error) {
	t.Helper()
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{}}
	for _, name := range missing {
		fake.Responses[name+" -m venv .venv"] = execx.Response{Err: &core.ToolMissingError{Tool: name}}
	}
	inv, err := Begin(execx.WithRunner(context.Background(), fake), core.Request{}, "python3")
	if err != nil {
		t.Fatal(err)
	}
	_, err = inv.RunProgram(true, "python3", "-m", "venv", ".venv")
	return fake, inv.Commands, err
}

func TestPythonWorksWithOnlyOneInterpreterName(t *testing.T) {
	cases := []struct {
		missing []string
		want    string
	}{
		{nil, "python3 -m venv .venv"},
		{[]string{"python3"}, "python -m venv .venv"},
		{[]string{"python3", "python"}, "py -m venv .venv"},
	}
	for _, c := range cases {
		_, cmds, err := run(t, c.missing...)
		if err != nil || !reflect.DeepEqual(cmds, []string{c.want}) {
			t.Errorf("missing %v: commands %v, err %v; want %q", c.missing, cmds, err, c.want)
		}
	}
}

func TestPythonMissingEverywhereReportsPython3(t *testing.T) {
	_, _, err := run(t, "python3", "python", "py")
	missing, ok := err.(*core.ToolMissingError)
	if !ok || missing.Tool != "python3" {
		t.Errorf("err = %v, want tool missing for python3", err)
	}
}
