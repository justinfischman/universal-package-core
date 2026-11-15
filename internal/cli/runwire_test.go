package cli

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/components/scripts"
	"github.com/justinfischman/universal-package-core/internal/core"
)

type noScripts struct{}

func (noScripts) ReadScripts(fs.FS) (scripts.Table, error) { return scripts.Table{}, fs.ErrNotExist }

// fakeSource is a RunSource that lists recipes through the runner and runs one
// as "recipe <name>", standing in for a concrete source (arch rule: cli may not
// import concrete modules; the real sources have their own tests).
type fakeSource struct{}

func (fakeSource) Recipes(ctx context.Context, dir string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(dir, "recipes.file")); err != nil {
		return nil, nil
	}
	out, err := execx.Ambient{}.Run(ctx, execx.Cmd{Name: "recipe", Args: []string{"--list"}, Dir: dir})
	return strings.Fields(out.Stdout), err
}

func (fakeSource) Run(ctx context.Context, dir, recipe string, args []string) error {
	_, err := execx.Ambient{}.Run(ctx, execx.Cmd{Name: "recipe", Args: append([]string{recipe}, args...), Dir: dir})
	return err
}

// fakeProvider is an UpProvider detecting "stack.file".
type fakeProvider struct{}

func (fakeProvider) Name() string { return "fakestack" }
func (fakeProvider) Detect(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "stack.file"))
	return err == nil
}
func (p fakeProvider) Up(ctx context.Context, t core.UpTarget) error   { return p.run(ctx, t, "up") }
func (p fakeProvider) Down(ctx context.Context, t core.UpTarget) error { return p.run(ctx, t, "down") }
func (fakeProvider) run(ctx context.Context, t core.UpTarget, verb string) error {
	_, err := execx.Ambient{}.Run(ctx, execx.Cmd{Name: "stack", Args: []string{verb}, Dir: t.Dir})
	return err
}

// wired returns a harness whose runner is a FakeRunner and whose registries
// carry the just run source and the stack provider up provider. The fake manager
// answers run and up as a manager with no matching manifest script would.
func wired(t *testing.T) (*harness, *execx.FakeRunner) {
	t.Helper()
	h := newHarness(t)
	f := &execx.FakeRunner{Responses: map[string]execx.Response{}}
	h.deps.NewRunner = func(*slog.Logger, bool, io.Writer, io.Writer) execx.Runner { return f }
	h.deps.Registries.RunSources = []core.RunSource{fakeSource{}}
	h.deps.Registries.UpProviders = []core.UpProvider{fakeProvider{}}
	return h, f
}

func (h *harness) missingFromManifest(t *testing.T) {
	t.Helper()
	_, runErr := scripts.Resolve(fstest.MapFS{}, "build", noScripts{})
	_, upErr := scripts.ResolveFirst(fstest.MapFS{}, scripts.UpNames, noScripts{})
	h.pm(t, "alpha-one").Hook = func(c core.Capability, _ core.Request) error {
		switch c {
		case core.CapRun:
			return runErr
		case core.CapUp:
			return upErr
		}
		return nil
	}
}

func (h *harness) writeProjectFile(t *testing.T, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.project, name), []byte("x:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func callNames(f *execx.FakeRunner) []string {
	var out []string
	for _, c := range f.Calls {
		out = append(out, c.String())
	}
	return out
}

func TestRunFallsBackToJustfileRecipe(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "build test\n"}}
	if code := h.run("run", "build"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if want := []string{"recipe --list", "recipe build"}; !reflect.DeepEqual(callNames(f), want) {
		t.Errorf("calls = %v, want %v", callNames(f), want)
	}
}

func TestRunManifestScriptOutranksJustfile(t *testing.T) {
	h, f := wired(t)
	h.writeProjectFile(t, "recipes.file")
	if code := h.run("run", "build"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(f.Calls) != 0 {
		t.Errorf("a manifest hit must not consult the recipe file: %v", callNames(f))
	}
}

func TestRunWithoutJustfileBehavesAsBefore(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	if code := h.run("run", "build"); code == core.ExitOK {
		t.Fatal("want the manager's not-declared error")
	}
	if len(f.Calls) != 0 {
		t.Errorf("no recipe file: nothing may run: %v", callNames(f))
	}
}

func TestRunRecipeNotInJustfileKeepsOriginalError(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "recipes.file")
	f.Responses["recipe --list"] = execx.Response{Output: execx.Output{Stdout: "test\n"}}
	if code := h.run("run", "build"); code == core.ExitOK {
		t.Fatal("want failure")
	}
	if len(f.Calls) != 1 {
		t.Errorf("only the listing may run: %v", callNames(f))
	}
}

func TestUpFallsBackToComposeAndDownUsesIt(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	h.writeProjectFile(t, "stack.file")
	if code := h.run("up"); code != core.ExitOK {
		t.Fatalf("up exit %d: %s", code, h.stderr)
	}
	want := []string{"stack up"}
	if !reflect.DeepEqual(callNames(f), want) {
		t.Fatalf("up calls = %v, want %v", callNames(f), want)
	}
	if got, _ := (runstate.State{Dir: h.project}).Provider(); got != "fakestack" {
		t.Errorf("recorded provider = %q", got)
	}
	f.Calls = nil
	if code := h.run("down"); code != core.ExitOK {
		t.Fatalf("down exit %d: %s", code, h.stderr)
	}
	want = []string{"stack down"}
	if !reflect.DeepEqual(callNames(f), want) {
		t.Errorf("down calls = %v, want %v", callNames(f), want)
	}
	if got, _ := (runstate.State{Dir: h.project}).Provider(); got != "" {
		t.Errorf("provider record not cleared: %q", got)
	}
	for _, c := range h.pm(t, "alpha-one").Calls {
		if c == core.CapDown {
			t.Error("down must go to stack provider, not the manager, when stack provider started it")
		}
	}
}

func TestUpScriptOutranksCompose(t *testing.T) {
	h, f := wired(t)
	h.writeProjectFile(t, "stack.file")
	if code := h.run("up"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(f.Calls) != 0 {
		t.Errorf("a manifest up script must not start stack provider: %v", callNames(f))
	}
	if code := h.run("down"); code != core.ExitOK {
		t.Fatalf("down exit %d: %s", code, h.stderr)
	}
	if len(f.Calls) != 0 {
		t.Errorf("down with no provider record goes to the manager: %v", callNames(f))
	}
}

func TestUpWithoutComposeFileKeepsOriginalError(t *testing.T) {
	h, f := wired(t)
	h.missingFromManifest(t)
	if code := h.run("up"); code == core.ExitOK {
		t.Fatal("want the manager's nothing-to-start error")
	}
	if len(f.Calls) != 0 {
		t.Errorf("calls = %v", callNames(f))
	}
}
