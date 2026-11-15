package cli

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/core"
)

type answerPrompter struct {
	prompt.NonInteractive
	answer bool
	asked  int
}

func (a *answerPrompter) Confirm(string, bool) (bool, error) { a.asked++; return a.answer, nil }

func TestPublishWithYesBuildsThenPublishes(t *testing.T) {
	h := newHarness(t)
	if code := h.run("publish", "--yes", "--", "--index", "x"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if want := []core.Capability{core.CapBuild, core.CapPublish}; !reflect.DeepEqual(pm.Calls, want) {
		t.Fatalf("calls = %v, want %v", pm.Calls, want)
	}
	if got := pm.Requests[1].Args; !reflect.DeepEqual(got, []string{"--index", "x"}) {
		t.Errorf("publish args = %v", got)
	}
	if pm.Requests[1].DryRun {
		t.Error("dry-run set on a real publish")
	}
}

func TestPublishRefusedWithoutYesWhenNonInteractive(t *testing.T) {
	h := newHarness(t)
	if code := h.run("publish"); code != core.ExitUsage {
		t.Fatalf("exit %d, want %d: %s", code, core.ExitUsage, h.stderr)
	}
	if !strings.Contains(h.stderr.String(), "--yes") {
		t.Errorf("stderr = %q, want a message naming --yes", h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if want := []core.Capability{core.CapBuild}; !reflect.DeepEqual(pm.Calls, want) {
		t.Errorf("calls = %v: publish must not run without confirmation", pm.Calls)
	}
}

func TestPublishDryRunNeedsNoConfirmationAndShowsPlan(t *testing.T) {
	h := newHarness(t)
	if code := h.run("publish", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if want := []core.Capability{core.CapBuild, core.CapPublish}; !reflect.DeepEqual(pm.Calls, want) {
		t.Fatalf("calls = %v", pm.Calls)
	}
	for _, r := range pm.Requests {
		if !r.DryRun {
			t.Errorf("request %+v lacks DryRun", r)
		}
	}
	out := h.stdout.String()
	if !strings.Contains(out, "$ alpha-one build") || !strings.Contains(out, "$ alpha-one publish") {
		t.Errorf("stdout = %q, want both planned commands", out)
	}
}

func TestPublishDeclinedPublishesNothing(t *testing.T) {
	h := newHarness(t)
	p := &answerPrompter{answer: false}
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return p }
	if code := h.run("publish"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if p.asked != 1 || !reflect.DeepEqual(pm.Calls, []core.Capability{core.CapBuild}) {
		t.Errorf("asked %d, calls %v", p.asked, pm.Calls)
	}
	if !strings.Contains(h.stdout.String(), "cancelled") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestPublishBuildFailureStopsBeforeAsking(t *testing.T) {
	h := newHarness(t)
	pm := h.pm(t, "alpha-one")
	pm.Hook = func(c core.Capability, _ core.Request) error {
		if c == core.CapBuild {
			return core.ErrInvalidRequest
		}
		return nil
	}
	p := &answerPrompter{answer: true}
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return p }
	if code := h.run("publish"); code == core.ExitOK {
		t.Fatal("want failure")
	}
	if p.asked != 0 || len(pm.Calls) != 1 {
		t.Errorf("asked %d, calls %v", p.asked, pm.Calls)
	}
}

func TestUpAndDownRouteToTheManager(t *testing.T) {
	h := newHarness(t)
	if code := h.run("up", "--", "--port", "1"); code != core.ExitOK {
		t.Fatalf("up exit %d: %s", code, h.stderr)
	}
	if code := h.run("down"); code != core.ExitOK {
		t.Fatalf("down exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if want := []core.Capability{core.CapUp, core.CapDown}; !reflect.DeepEqual(pm.Calls, want) {
		t.Fatalf("calls = %v", pm.Calls)
	}
	if got := pm.Requests[0].Args; !reflect.DeepEqual(got, []string{"--port", "1"}) {
		t.Errorf("up args = %v", got)
	}
}
