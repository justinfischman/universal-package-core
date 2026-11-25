package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// failingPrompter fails every Choose with err, as a closed terminal would.
type failingPrompter struct {
	prompt.NonInteractive
	err error
}

func (p failingPrompter) Choose(string, []string) (string, error) { return "", p.err }

func TestUpOfferReturnsAPromptFailure(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "dev\n"}}
	p := failingPrompter{err: errors.New("input closed")}
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return p }
	if code := h.run("up"); code == core.ExitOK {
		t.Fatal("want failure when the prompt fails")
	}
	if !strings.Contains(h.stderr.String(), "input closed") {
		t.Errorf("stderr lacks the prompt error: %s", h.stderr.String())
	}
}
