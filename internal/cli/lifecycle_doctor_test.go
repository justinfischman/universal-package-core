package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

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
	golden(t, "doctor.json", []byte(redactProjectDir(h.stdout.String(), h.project)))
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

// redactProjectDir replaces the project directory in JSON output. On Windows
// the directory appears with escaped backslashes, so that form goes first.
func redactProjectDir(out, dir string) string {
	escaped := strings.ReplaceAll(dir, `\`, `\\`)
	return strings.ReplaceAll(strings.ReplaceAll(out, escaped, "<project>"), dir, "<project>")
}
