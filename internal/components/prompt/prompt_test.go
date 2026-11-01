package prompt_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestNonInteractive(t *testing.T) {
	yes := prompt.NonInteractive{Yes: true}
	if ok, err := yes.Confirm("go?", false); err != nil || !ok {
		t.Errorf("yes: %v %v", ok, err)
	}
	no := prompt.NonInteractive{}
	_, err := no.Confirm("go?", true)
	if !errors.Is(err, core.ErrInvalidRequest) || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("want error naming --yes, got %v", err)
	}
	if _, err := yes.Choose("pick", []string{"a"}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("choose: %v", err)
	}
}

func TestTerminalConfirm(t *testing.T) {
	cases := []struct {
		input string
		def   bool
		want  bool
		bad   bool
	}{
		{"y\n", false, true, false},
		{"YES\n", false, true, false},
		{"n\n", true, false, false},
		{"\n", true, true, false},
		{"\n", false, false, false},
		{"maybe\n", false, false, true},
	}
	for _, c := range cases {
		term := prompt.Terminal{In: strings.NewReader(c.input), Out: &bytes.Buffer{}}
		got, err := term.Confirm("ok?", c.def)
		if (err != nil) != c.bad || got != c.want {
			t.Errorf("%q def=%v: got %v, %v", c.input, c.def, got, err)
		}
	}
}

func TestTerminalChoose(t *testing.T) {
	opts := []string{"uv", "pip", "poetry"}
	term := prompt.Terminal{In: strings.NewReader("2\n"), Out: &bytes.Buffer{}}
	if got, err := term.Choose("pm", opts); err != nil || got != "pip" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, in := range []string{"0\n", "4\n", "x\n"} {
		term := prompt.Terminal{In: strings.NewReader(in), Out: &bytes.Buffer{}}
		if _, err := term.Choose("pm", opts); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%q: %v", in, err)
		}
	}
}

func TestNewSelectsNonInteractive(t *testing.T) {
	if _, ok := prompt.New(true, nil, &bytes.Buffer{}).(prompt.NonInteractive); !ok {
		t.Error("--yes should be NonInteractive")
	}
	if _, ok := prompt.New(false, nil, &bytes.Buffer{}).(prompt.NonInteractive); !ok {
		t.Error("no input should be NonInteractive")
	}
}
