package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

type harness struct {
	deps    Deps
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	home    string // UPC_HOME
	project string // cwd
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	h := &harness{
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
		home:    filepath.Join(base, "home"),
		project: filepath.Join(base, "proj"),
	}
	if err := os.MkdirAll(h.project, 0o755); err != nil {
		t.Fatal(err)
	}
	// A package manager marker makes the directory a discoverable project.
	if err := os.WriteFile(filepath.Join(h.project, "alpha-one.marker"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	d := DefaultDeps(regs, BuildInfo{Version: "1.2.3", Commit: "abc", Date: "today"})
	d.Stdin = nil // never a terminal in tests
	d.Stdout, d.Stderr = h.stdout, h.stderr
	d.Getenv = func(k string) string {
		if k == config.HomeEnv {
			return h.home
		}
		return ""
	}
	d.Getwd = func() (string, error) { return h.project, nil }
	h.deps = d
	return h
}

func (h *harness) run(args ...string) int {
	h.stdout.Reset()
	h.stderr.Reset()
	return Execute(context.Background(), h.deps, args)
}

func TestVersionText(t *testing.T) {
	h := newHarness(t)
	if code := h.run("version"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if got := strings.TrimSpace(h.stdout.String()); got != "upc 1.2.3" {
		t.Errorf("stdout = %q", got)
	}
}

func TestVersionJSON(t *testing.T) {
	h := newHarness(t)
	if code := h.run("version", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	var out struct {
		Summary string
		Data    BuildInfo
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil {
		t.Fatalf("stdout not json: %v: %s", err, h.stdout)
	}
	if out.Data.Version != "1.2.3" || out.Data.Commit != "abc" {
		t.Errorf("data = %+v", out.Data)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"nonsense"},
		{"version", "--nope"},
		{"version", "extra"},
		{"--global", "--project", "x", "version"},
	} {
		if code := h.run(args...); code != core.ExitUsage {
			t.Errorf("%v: exit %d, stderr %q", args, code, h.stderr)
		}
		if !strings.HasPrefix(h.stderr.String(), "upc: ") || strings.Count(h.stderr.String(), "upc: ") != 1 {
			t.Errorf("%v: error not printed exactly once: %q", args, h.stderr)
		}
	}
}

func TestVerboseFlagReachesLogger(t *testing.T) {
	h := newHarness(t)
	h.run("version")
	if strings.Contains(h.stderr.String(), "session") {
		t.Errorf("debug log without --verbose: %s", h.stderr)
	}
	h.run("version", "--verbose")
	if !strings.Contains(h.stderr.String(), "session") {
		t.Errorf("--verbose produced no debug log: %q", h.stderr)
	}
}

func TestJSONFlagMakesLogsJSON(t *testing.T) {
	h := newHarness(t)
	h.run("version", "--json", "--verbose")
	line := strings.SplitN(strings.TrimSpace(h.stderr.String()), "\n", 2)[0]
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Errorf("log line is not json: %q", line)
	}
}

// Regression for the cobra hook gotcha: without EnableTraverseRunHooks a child
// that defines its own PersistentPreRunE would skip the root's, leaving no session.
func TestRootHooksRunForChildWithOwnHook(t *testing.T) {
	h := newHarness(t)
	root := NewRoot(h.deps)
	var sawSession, childHook bool
	root.AddCommand(&cobra.Command{
		Use:               "probe",
		PersistentPreRunE: func(*cobra.Command, []string) error { childHook = true; return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := sessionFrom(cmd)
			sawSession = err == nil
			return err
		},
	})
	root.SetArgs([]string{"probe"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !childHook || !sawSession {
		t.Errorf("childHook=%v sawSession=%v", childHook, sawSession)
	}
}

func TestConfigSetAndGet(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "set", "language", "alpha"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	path := filepath.Join(h.project, config.DirName, config.FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("project config not written: %v", err)
	}
	if code := h.run("config", "get", "language"); code != 0 || !strings.Contains(h.stdout.String(), "language = alpha (project)") {
		t.Errorf("exit %d, stdout %q", code, h.stdout)
	}
}

func TestConfigSetRefusesCrossLanguageSwitch(t *testing.T) {
	h := newHarness(t)
	for _, kv := range [][2]string{{"language", "alpha"}, {"package_manager", "alpha-one"}} {
		if code := h.run("config", "set", kv[0], kv[1]); code != 0 {
			t.Fatalf("setup %v: exit %d: %s", kv, code, h.stderr)
		}
	}
	path := filepath.Join(h.project, config.DirName, config.FileName)
	before, _ := os.ReadFile(path)

	if code := h.run("config", "set", "package_manager", "beta-one"); code != core.ExitUsage {
		t.Fatalf("exit %d, want %d", code, core.ExitUsage)
	}
	if !strings.Contains(h.stderr.String(), "belongs to language") {
		t.Errorf("unhelpful message: %q", h.stderr)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Errorf("config changed despite refusal:\n%s\n--\n%s", before, after)
	}
}

func TestConfigSetWithinLanguageAllowed(t *testing.T) {
	h := newHarness(t)
	h.run("config", "set", "language", "alpha")
	h.run("config", "set", "package_manager", "alpha-one")
	if code := h.run("config", "set", "package_manager", "alpha-two"); code != 0 {
		t.Errorf("switch within language refused: %s", h.stderr)
	}
}

func TestConfigSetGlobalAndMergedValidation(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "set", "--global", "language", "alpha"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.home, config.FileName)); err != nil {
		t.Errorf("global config not written under UPC_HOME: %v", err)
	}
	// Project scope must be validated against the global language.
	if code := h.run("config", "set", "package_manager", "beta-one"); code != core.ExitUsage {
		t.Errorf("merged validation: exit %d", code)
	}
}

func TestConfigSetWithoutProjectIsUsageError(t *testing.T) {
	h := newHarness(t)
	h.deps.Getwd = func() (string, error) { return "/", nil }
	h.deps.FS = noRootFS{h.deps.FS}
	if code := h.run("config", "set", "language", "alpha"); code != core.ExitUsage {
		t.Errorf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stderr.String(), "--global") {
		t.Errorf("message should mention --global: %q", h.stderr)
	}
}

func TestConfigSetDryRunWritesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "set", "--dry-run", "language", "alpha"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.project, config.DirName)); err == nil {
		t.Error("dry-run wrote config")
	}
	if !strings.Contains(h.stdout.String(), "would set") {
		t.Errorf("stdout = %q", h.stdout)
	}
}

func TestConfigSetUnknownKeyAndValue(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "set", "bogus", "x"); code != core.ExitUsage {
		t.Errorf("unknown key: exit %d", code)
	}
	if code := h.run("config", "set", "language", "nope"); code != core.ExitUsage {
		t.Errorf("unknown language: exit %d", code)
	}
}

func TestConfigValidate(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "validate"); code != 0 || !strings.Contains(h.stdout.String(), "valid") {
		t.Errorf("empty config: exit %d, %q", code, h.stdout)
	}
	// Hand-write an invalid file; `set` can't produce it.
	dir := filepath.Join(h.project, config.DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("language = \"alpha\"\npackage_manager = \"beta-one\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run("config", "validate"); code != core.ExitUsage {
		t.Errorf("invalid config: exit %d, %q", code, h.stderr)
	}
}

func TestConfigGetUnsetShowsPlaceholder(t *testing.T) {
	h := newHarness(t)
	h.run("config", "get")
	if !strings.Contains(h.stdout.String(), "language = (unset)") {
		t.Errorf("stdout = %q", h.stdout)
	}
}
