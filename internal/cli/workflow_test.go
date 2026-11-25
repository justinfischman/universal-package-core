package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/prompt"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

// pickingPrompter answers every Choose with pick and records the options.
type pickingPrompter struct {
	prompt.NonInteractive
	pick    string
	options []string
}

func (p *pickingPrompter) Choose(_ string, options []string) (string, error) {
	p.options = options
	return p.pick, nil
}

func TestPublishNativeDryRunBuildsThenChecksWithoutConfirming(t *testing.T) {
	h := newHarness(t)
	h.pm(t, "alpha-one").PublishCheckArgs = []string{"--check"}
	// Non-interactive and no --yes: a real publish would be refused.
	if code := h.run("publish", "--native-dry-run", "--", "--index", "x"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if want := []core.Capability{core.CapBuild, core.CapPublish}; !reflect.DeepEqual(pm.Calls, want) {
		t.Fatalf("calls = %v, want %v", pm.Calls, want)
	}
	if got := pm.Requests[1].Args; !reflect.DeepEqual(got, []string{"--index", "x", "--check"}) {
		t.Errorf("publish args = %v", got)
	}
	if pm.Requests[1].DryRun {
		t.Error("the native check must really run, not be planned")
	}
}

func TestPublishNativeDryRunWithGlobalDryRunPrintsThePlan(t *testing.T) {
	h := newHarness(t)
	h.pm(t, "alpha-one").PublishCheckArgs = []string{"--check"}
	if code := h.run("publish", "--native-dry-run", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	for _, r := range pm.Requests {
		if !r.DryRun {
			t.Errorf("request %+v lacks DryRun", r)
		}
	}
	if got := pm.Requests[1].Args; !reflect.DeepEqual(got, []string{"--check"}) {
		t.Errorf("planned publish args = %v", got)
	}
}

func TestPublishNativeDryRunRefusedWhenManagerHasNone(t *testing.T) {
	h := newHarness(t) // the fake offers no publish check by default
	if code := h.run("publish", "--native-dry-run"); code != core.ExitUsage {
		t.Fatalf("exit %d, want %d: %s", code, core.ExitUsage, h.stderr)
	}
	if pm := h.pm(t, "alpha-one"); len(pm.Calls) != 0 {
		t.Errorf("nothing may run: %v", pm.Calls)
	}
}

func TestUpAppStartsItsDependenciesFirstAndDownStopsOnlyIt(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, twoAppConfig)
	if code := h.run("up", "--app", "web"); code != core.ExitOK {
		t.Fatalf("up exit %d: %s", code, h.stderr)
	}
	if code := h.run("down", "--app", "web"); code != core.ExitOK {
		t.Fatalf("down exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	var got []string
	for i, r := range pm.Requests {
		got = append(got, string(pm.Calls[i])+" "+r.Member)
	}
	if want := []string{"up api", "up web", "down web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

func TestUpOffersRecipesAndSavesTheChosenOne(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "dev test\n"}}
	p := &pickingPrompter{pick: "dev"}
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return p }
	if code := h.run("up"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if want := []string{"dev", "test", skipUpOffer}; !reflect.DeepEqual(p.options, want) {
		t.Errorf("options = %v, want %v", p.options, want)
	}
	body, err := os.ReadFile(filepath.Join(h.project, config.DirName, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "up = ['upc', 'run', 'dev']") {
		t.Errorf("config does not hold the up command:\n%s", body)
	}
	if len(f.Procs) != 1 || f.Procs[0].Cmd.String() != "upc run dev" {
		t.Errorf("procs = %+v", f.Procs)
	}
}

func TestUpOfferDeclinedKeepsTheOriginalErrorAndWritesNothing(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "dev\n"}}
	p := &pickingPrompter{pick: skipUpOffer}
	h.deps.NewPrompter = func(bool, *os.File, io.Writer) prompt.Prompter { return p }
	if code := h.run("up"); code == core.ExitOK {
		t.Fatal("want failure when the offer is declined")
	}
	if _, err := os.Stat(filepath.Join(h.project, config.DirName, config.FileName)); err == nil {
		t.Error("declined offer wrote config")
	}
	if len(f.Procs) != 0 {
		t.Errorf("started something: %+v", f.Procs)
	}
}

func TestUpOfferNeedsAnInteractiveSession(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "dev\n"}}
	if code := h.run("up", "--yes"); code == core.ExitOK {
		t.Fatal("a non-interactive up must not invent a command")
	}
	if _, err := os.Stat(filepath.Join(h.project, config.DirName, config.FileName)); err == nil {
		t.Error("non-interactive run wrote config")
	}
}

func TestWorkspaceCheckFailsOnMemberMismatchAndPassesOnMatch(t *testing.T) {
	h, _ := contractHarness(t)
	h.writeProjectConfig(t, h.project, "[workspace]\nmembers = [\"apps/api\", \"apps/web\"]\n"+contractAppConfig)
	h.pm(t, "alpha-one").Members = []string{"apps/web", "apps/other"}
	if code := h.run("workspace", "check", "--update"); code == core.ExitOK {
		t.Fatalf("want failure on a member mismatch:\n%s", h.stdout.String())
	}
	out := h.stdout.String()
	for _, want := range []string{"members: mismatch", "apps/api", "apps/other"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	h.pm(t, "alpha-one").Members = []string{"apps/web/", "apps/api"}
	if code := h.run("workspace", "check"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "members: ok") {
		t.Errorf("output = %s", h.stdout.String())
	}
}
