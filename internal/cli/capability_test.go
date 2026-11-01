package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

var update = flag.Bool("update", false, "rewrite golden files")

// golden compares got with testdata/<name>, or rewrites it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs\n got: %s\nwant: %s", name, got, want)
	}
}

// pm returns the fake manager registered under id.
func (h *harness) pm(t *testing.T, id string) *coretest.PackageManager {
	t.Helper()
	m, ok := h.deps.Registries.PackageManagers.Get(id)
	if !ok {
		t.Fatalf("no package manager %q", id)
	}
	fake, ok := m.(*coretest.PackageManager)
	if !ok {
		t.Fatalf("%q is not a fake", id)
	}
	return fake
}

// useRunner swaps the runner every session gets.
func (h *harness) useRunner(r execx.Runner) {
	h.deps.NewRunner = func(*slog.Logger, bool, io.Writer, io.Writer) execx.Runner { return r }
}

func TestAddRoutesToDetectedManager(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add", "foo", "bar", "--dev"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	want := core.Request{Dir: h.project, Packages: []string{"foo", "bar"}, Group: "dev"}
	if len(pm.Requests) != 1 || !reflect.DeepEqual(pm.Requests[0], want) {
		t.Errorf("requests = %+v, want %+v", pm.Requests, want)
	}
	if got := strings.TrimSpace(h.stdout.String()); got != "alpha-one add" {
		t.Errorf("stdout = %q (routed commands must stay hidden outside dry-run)", got)
	}
}

func TestAddDryRunShowsRoutedCommand(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add", "foo", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !h.pm(t, "alpha-one").Requests[0].DryRun {
		t.Error("dry-run flag did not reach the request")
	}
	if !strings.Contains(h.stdout.String(), "$ alpha-one add") {
		t.Errorf("stdout = %q, want the routed command", h.stdout)
	}
}

func TestAddJSONGolden(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add", "foo", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	golden(t, "add.json", h.stdout.Bytes())
}

func TestAddRequiresAPackage(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add"); code != core.ExitUsage {
		t.Errorf("exit %d, want usage error", code)
	}
}

func TestFlagsAfterRunScriptArePassedThrough(t *testing.T) {
	h := newHarness(t)
	if code := h.run("run", "tool", "--flag", "x"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	got := h.pm(t, "alpha-one").Requests[0]
	if got.Script != "tool" || !reflect.DeepEqual(got.Args, []string{"--flag", "x"}) {
		t.Errorf("request = %+v", got)
	}
}

func TestUnsupportedCapabilityExitsThree(t *testing.T) {
	h := newHarness(t)
	pm, err := coretest.NewPackageManager("alpha-one", "alpha", core.CapAdd)
	if err != nil {
		t.Fatal(err)
	}
	regs := core.NewRegistries()
	lang, _ := h.deps.Registries.Languages.Get("alpha")
	_ = regs.Languages.Register(lang)
	_ = regs.PackageManagers.Register(pm)
	h.deps.Registries = regs
	if code := h.run("add", "foo"); code != core.ExitUnsupported {
		t.Errorf("exit %d, want %d: %s", code, core.ExitUnsupported, h.stderr)
	}
	if !strings.Contains(h.stderr.String(), "alpha-one") || !strings.Contains(h.stderr.String(), "add") {
		t.Errorf("stderr should name manager and capability: %q", h.stderr)
	}
}

func TestProjectCommandsRefuseGlobalScope(t *testing.T) {
	h := newHarness(t)
	if code := h.run("add", "foo", "--global"); code != core.ExitUsage {
		t.Errorf("exit %d, want usage", code)
	}
}

func TestIncompatibleConfigIsRejected(t *testing.T) {
	h := newHarness(t)
	cfgDir := filepath.Join(h.project, ".universal-package-core")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "language = \"alpha\"\npackage_manager = \"beta-one\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("add", "foo"); code != core.ExitUsage {
		t.Errorf("exit %d, want usage: %s", code, h.stderr)
	}
	if len(h.pm(t, "alpha-one").Calls) != 0 {
		t.Error("nothing may run under an incompatible config")
	}
}

func TestConfigChoosesManagerOverDetection(t *testing.T) {
	h := newHarness(t)
	cfgDir := filepath.Join(h.project, ".universal-package-core")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("package_manager = \"alpha-two\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("add", "foo"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(h.pm(t, "alpha-two").Calls) != 1 || len(h.pm(t, "alpha-one").Calls) != 0 {
		t.Error("config must select alpha-two")
	}
}

func TestNothingDetectedIsAUsageError(t *testing.T) {
	h := newHarness(t)
	h.deps.Getwd = func() (string, error) { return t.TempDir(), nil }
	if code := h.run("add", "foo"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stderr.String(), "upc init") {
		t.Errorf("stderr should point to init/new: %q", h.stderr)
	}
}

// missingTool makes the fake manager report its tool missing until installed.
type missingTool struct {
	pm        *coretest.PackageManager
	installed bool
	calls     []execx.Cmd
	fail      error
}

func (m *missingTool) Run(_ context.Context, cmd execx.Cmd) (execx.Output, error) {
	m.calls = append(m.calls, cmd)
	if cmd.Name == "install" {
		m.installed = m.fail == nil
		return execx.Output{}, m.fail
	}
	return execx.Output{}, nil
}

func (m *missingTool) hook(core.Capability, core.Request) error {
	if m.installed {
		return nil
	}
	return &core.ToolMissingError{Tool: "alpha-one"}
}

func newMissingToolHarness(t *testing.T) (*harness, *missingTool) {
	t.Helper()
	h := newHarness(t)
	m := &missingTool{pm: h.pm(t, "alpha-one")}
	m.pm.Hook = m.hook
	h.useRunner(m)
	return h, m
}

func TestMissingToolWithYesInstallsAndRerunsOriginalCommand(t *testing.T) {
	h, m := newMissingToolHarness(t)
	if code := h.run("add", "foo", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(m.calls) != 1 || m.calls[0].String() != "install alpha-one" {
		t.Errorf("install calls = %+v", m.calls)
	}
	if got := len(m.pm.Calls); got != 2 {
		t.Errorf("handler ran %d times, want 2 (failed, then re-run after install)", got)
	}
}

func TestMissingToolNonInteractiveFailsWithInstructions(t *testing.T) {
	h, m := newMissingToolHarness(t)
	code := h.run("add", "foo")
	if code != core.ExitToolMissing {
		t.Fatalf("exit %d, want %d: %s", code, core.ExitToolMissing, h.stderr)
	}
	for _, want := range []string{"alpha-one", "install alpha-one", "--yes"} {
		if !strings.Contains(h.stderr.String(), want) {
			t.Errorf("stderr %q lacks %q", h.stderr, want)
		}
	}
	if len(m.calls) != 0 {
		t.Errorf("must not install without consent: %+v", m.calls)
	}
}

func TestMissingToolInstallFailureIsReported(t *testing.T) {
	h, m := newMissingToolHarness(t)
	m.fail = errors.New("boom")
	code := h.run("add", "foo", "--yes")
	if code == core.ExitOK || !strings.Contains(h.stderr.String(), "boom") {
		t.Errorf("exit %d stderr %q", code, h.stderr)
	}
}

func TestJSONModeRoutesToolOutputToStderr(t *testing.T) {
	h := newHarness(t)
	var gotStdout io.Writer
	h.deps.NewRunner = func(_ *slog.Logger, _ bool, out, _ io.Writer) execx.Runner {
		gotStdout = out
		return &execx.FakeRunner{}
	}
	h.run("add", "foo", "--json")
	if gotStdout != io.Writer(h.stderr) {
		t.Error("with --json the tool's stdout must go to stderr so stdout stays one JSON document")
	}
	var doc map[string]any
	if err := json.Unmarshal(h.stdout.Bytes(), &doc); err != nil {
		t.Errorf("stdout is not a single JSON document: %v", err)
	}
}
