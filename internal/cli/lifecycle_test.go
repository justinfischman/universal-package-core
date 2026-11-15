package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func (h *harness) writeProjectConfig(t *testing.T, dir, body string) {
	t.Helper()
	path := filepath.Join(dir, config.DirName, config.FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, dir string) (config.Config, bool) {
	t.Helper()
	store := config.Store{FS: fsx.OS{}, GlobalDir: filepath.Join(dir, "no-global"), ProjectRoot: dir}
	if _, err := os.Stat(filepath.Join(dir, config.DirName, config.FileName)); err != nil {
		return config.Config{}, false
	}
	c, err := store.LoadScope(config.Project)
	if err != nil {
		t.Fatal(err)
	}
	return c, true
}

func TestNewYesPicksDefaultsAndWritesProjectConfig(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "thing", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, filepath.Join(h.project, "thing"))
	want := config.Config{Language: "alpha", PackageManager: "alpha-one", ProjectType: "gadget"}
	if !ok || cfg.Language != want.Language || cfg.PackageManager != want.PackageManager || cfg.ProjectType != want.ProjectType {
		t.Errorf("config = %+v (present %v), want %+v", cfg, ok, want)
	}
	if !strings.Contains(h.stdout.String(), "scaffolded thing") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestNewAcceptsNameFlagAndExplicitStack(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "--name", "x", "--language", "beta"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "x")); cfg.PackageManager != "beta-one" {
		t.Errorf("config = %+v", cfg)
	}
	if code := h.run("new", "gadget", "y", "--pm", "alpha-two"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "y")); cfg.Language != "alpha" || cfg.PackageManager != "alpha-two" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestNewUsesConfiguredStack(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "language = \"beta\"\n")
	if code := h.run("new", "gadget", "z"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if cfg, _ := readConfig(t, filepath.Join(h.project, "z")); cfg.Language != "beta" {
		t.Errorf("config = %+v", cfg)
	}
}

func TestNewUsageErrors(t *testing.T) {
	h := newHarness(t)
	cases := map[string][]string{
		"unknown type":            {"new", "nonsense", "x", "--yes"},
		"missing name":            {"new", "gadget", "--yes"},
		"no type":                 {"new"},
		"name twice":              {"new", "gadget", "a", "--name", "b", "--yes"},
		"language/pm mismatch":    {"new", "gadget", "x", "--language", "beta", "--pm", "alpha-one"},
		"unknown language":        {"new", "gadget", "x", "--language", "cobol"},
		"unknown pm":              {"new", "gadget", "x", "--pm", "nope"},
		"ambiguous non-interact.": {"new", "gadget", "x"},
		"global scope":            {"new", "gadget", "x", "--yes", "--global"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code := h.run(args...); code != core.ExitUsage {
				t.Errorf("%v: exit %d, stderr %q", args, code, h.stderr)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(h.project, "x")); err == nil {
		t.Error("a refused command must not create the project")
	}
}

func TestNewAmbiguityMessageNamesTheFlag(t *testing.T) {
	h := newHarness(t)
	h.run("new", "gadget", "x")
	if !strings.Contains(h.stderr.String(), "--language") {
		t.Errorf("stderr = %q, want a hint to pass --language", h.stderr)
	}
}

func TestNewDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("new", "gadget", "thing", "--yes", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.project, "thing")); err == nil {
		t.Error("dry run created the project")
	}
}

func TestInitWritesConfigAndInstalls(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, h.project)
	if !ok || cfg.Language != "alpha" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v (present %v)", cfg, ok)
	}
	pm := h.pm(t, "alpha-one")
	if len(pm.Calls) != 1 || pm.Calls[0] != core.CapInstall || pm.Requests[0].Dir != h.project {
		t.Errorf("calls = %v", pm.Calls)
	}
	var out struct {
		Data initSummary
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Data.ConfigWritten || !out.Data.Installed || out.Data.Source != sourceDetected {
		t.Errorf("summary = %+v", out.Data)
	}
}

func TestInitKeepsExistingChoice(t *testing.T) {
	h := newHarness(t)
	body := "package_manager = \"alpha-two\"\n"
	h.writeProjectConfig(t, h.project, body)
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	got, _ := os.ReadFile(filepath.Join(h.project, config.DirName, config.FileName))
	if string(got) != body {
		t.Errorf("config changed to %q", got)
	}
	if len(h.pm(t, "alpha-two").Calls) != 1 {
		t.Error("install must run through the configured manager")
	}
}

func TestInitFillsConfigWithoutStackAfterConfirmation(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "project_type = \"gadget\"\n")
	if code := h.run("init"); code == core.ExitOK {
		t.Fatal("non-interactive without --yes must not overwrite silently")
	}
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	if cfg.ProjectType != "gadget" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v, want type preserved and manager added", cfg)
	}
}

func TestInitDoesNotTouchManifests(t *testing.T) {
	h := newHarness(t)
	marker := filepath.Join(h.project, "alpha-one.marker")
	before, _ := os.ReadFile(marker)
	h.run("init", "--yes")
	after, _ := os.ReadFile(marker)
	if string(before) != string(after) {
		t.Error("init modified a project file")
	}
	entries, _ := os.ReadDir(h.project)
	for _, e := range entries {
		if e.Name() != "alpha-one.marker" && e.Name() != config.DirName && e.Name() != ".gitignore" {
			t.Errorf("init created %q", e.Name())
		}
	}
}

func TestInitDryRunChangesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("init", "--yes", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, h.project); ok {
		t.Error("dry run wrote config")
	}
	if !h.pm(t, "alpha-one").Requests[0].DryRun {
		t.Error("install must be a dry run")
	}
}

func TestInitInEmptyDirectoryWalksThroughChoicesWithYes(t *testing.T) {
	h := newHarness(t)
	empty := t.TempDir()
	h.deps.Getwd = func() (string, error) { return empty, nil }
	if code := h.run("init", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, ok := readConfig(t, empty)
	if !ok || cfg.Language != "alpha" || cfg.PackageManager != "alpha-one" {
		t.Errorf("config = %+v", cfg)
	}
	if len(h.pm(t, "alpha-one").Calls) != 0 {
		t.Error("nothing exists to install yet")
	}
	if !strings.Contains(h.stdout.String(), "upc new") {
		t.Errorf("stdout = %q should point to `upc new`", h.stdout)
	}
}

func TestInitInEmptyDirectoryNonInteractiveFails(t *testing.T) {
	h := newHarness(t)
	empty := t.TempDir()
	h.deps.Getwd = func() (string, error) { return empty, nil }
	if code := h.run("init"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
	if _, ok := readConfig(t, empty); ok {
		t.Error("nothing may be written when the walkthrough cannot run")
	}
}

func TestInitRefusesIncompatibleConfig(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "language = \"alpha\"\npackage_manager = \"beta-one\"\n")
	if code := h.run("init", "--yes"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
}

// probeRunner answers the tool's version probe and records everything it ran.
type probeRunner struct {
	version string
	missing bool
	calls   []string
}

func (p *probeRunner) Run(_ context.Context, cmd execx.Cmd) (execx.Output, error) {
	p.calls = append(p.calls, cmd.String())
	if cmd.String() == "alpha-one --version" {
		if p.missing {
			return execx.Output{}, &core.ToolMissingError{Tool: "alpha-one"}
		}
		return execx.Output{Stdout: p.version + "\n"}, nil
	}
	if cmd.Name == "install" {
		p.missing = false
	}
	return execx.Output{}, nil
}

func TestDoctorReportsToolStackAndCapabilities(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "alpha-one 9.9"})
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	for _, want := range []string{"stack: alpha / alpha-one (detected)", "tool alpha-one: found alpha-one 9.9", "capabilities:", "add", "native", "findings: none"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output lacks %q:\n%s", want, out)
		}
	}
}

func TestDoctorShowsUnsupportedReasons(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "1"})
	h.run("doctor", "--json")
	var out struct {
		Data doctorReport
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Data.Capabilities) != len(core.AllCapabilities) {
		t.Errorf("matrix has %d entries, want %d", len(out.Data.Capabilities), len(core.AllCapabilities))
	}
}

func TestDoctorJSONGolden(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "alpha-one 9.9"})
	if code := h.run("doctor", "--json"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	golden(t, "doctor.json", []byte(strings.ReplaceAll(h.stdout.String(), h.project, "<project>")))
}

func TestDoctorMissingToolIsOfferedAndFixedWithYes(t *testing.T) {
	h := newHarness(t)
	p := &probeRunner{missing: true, version: "alpha-one 1"}
	h.useRunner(p)
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "not installed") || strings.Contains(p.callsJoined(), "install alpha-one") {
		t.Errorf("without --yes the tool must only be reported:\n%s\ncalls: %v", h.stdout, p.calls)
	}

	if code := h.run("doctor", "--yes"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(p.callsJoined(), "install alpha-one") || !strings.Contains(h.stdout.String(), "[fixed]") {
		t.Errorf("--yes must install:\n%s\ncalls: %v", h.stdout, p.calls)
	}
}

func (p *probeRunner) callsJoined() string { return strings.Join(p.calls, "|") }

func TestDoctorReportsConfigDrift(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "1"})
	h.writeProjectConfig(t, h.project, "package_manager = \"alpha-two\"\n")
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "config says alpha-two") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestDoctorRejectsIncompatibleConfig(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "language = \"alpha\"\npackage_manager = \"beta-one\"\n")
	if code := h.run("doctor"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
}

func TestDoctorWithNothingDetectedReports(t *testing.T) {
	h := newHarness(t)
	h.deps.Getwd = func() (string, error) { return t.TempDir(), nil }
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "no supported project") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestDoctorDryRunDoesNotProbe(t *testing.T) {
	h := newHarness(t)
	p := &probeRunner{version: "1"}
	h.useRunner(p)
	if code := h.run("doctor", "--dry-run"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if len(p.calls) != 0 || !strings.Contains(h.stdout.String(), "not checked") {
		t.Errorf("calls %v, stdout %q", p.calls, h.stdout)
	}
}
