package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestGenericCommandsRouteTheirCapability(t *testing.T) {
	cases := []struct {
		cmd  string
		cap  core.Capability
		args []string
		want []string
	}{
		{"build", core.CapBuild, nil, nil},
		{"test", core.CapTest, []string{"--", "-k", "fast"}, []string{"-k", "fast"}},
		{"lint", core.CapLint, []string{"src"}, []string{"src"}},
		{"audit", core.CapAudit, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.cmd, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(append([]string{c.cmd}, c.args...)...); code != core.ExitOK {
				t.Fatalf("exit %d: %s", code, h.stderr)
			}
			reqs := h.pm(t, "alpha-one").Requests
			if len(reqs) != 1 || reqs[0].Dir != h.project || strings.Join(reqs[0].Args, " ") != strings.Join(c.want, " ") {
				t.Errorf("requests = %+v", reqs)
			}
			if got := strings.TrimSpace(h.stdout.String()); got != "alpha-one "+string(c.cap) {
				t.Errorf("stdout = %q", got)
			}
		})
	}
}

func TestConfigSetPMSwitchesWithinALanguage(t *testing.T) {
	h := newHarness(t)
	h.writeProjectConfig(t, h.project, "package_manager = \"alpha-one\"\n")
	if code := h.run("config", "set", "pm", "alpha-two"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	if cfg.PackageManager != "alpha-two" {
		t.Errorf("config = %+v", cfg)
	}
	out := h.stdout.String()
	if !strings.Contains(out, "set pm = alpha-two") || !strings.Contains(out, "does not convert manifests or lockfiles") || !strings.Contains(out, "upc doctor") {
		t.Errorf("stdout = %q", out)
	}
	// Setting the same manager again carries no warning.
	if code := h.run("config", "set", "pm", "alpha-two"); code != core.ExitOK || strings.Contains(h.stdout.String(), "does not convert") {
		t.Errorf("exit %d, stdout = %q", code, h.stdout)
	}
	if code := h.run("config", "get", "pm"); code != core.ExitOK || !strings.Contains(h.stdout.String(), "pm = alpha-two (project)") {
		t.Errorf("exit %d, stdout = %q", code, h.stdout)
	}
}

func TestConfigSetPMRefusesAnotherLanguage(t *testing.T) {
	cases := map[string]string{
		"configured manager":  "package_manager = \"alpha-one\"\n",
		"configured language": "language = \"alpha\"\n",
		"detected from files": "",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			if body != "" {
				h.writeProjectConfig(t, h.project, body)
			}
			for _, args := range [][]string{{"pm", "beta-one"}, {"package_managers", "alpha-one,beta-one"}} {
				if code := h.run(append([]string{"config", "set"}, args...)...); code != core.ExitUsage {
					t.Fatalf("%v: exit %d, want 2: %s", args, code, h.stderr)
				}
				if !strings.Contains(h.stderr.String(), "only allowed within a language") {
					t.Errorf("stderr = %q", h.stderr)
				}
			}
			if cfg, _ := readConfig(t, h.project); cfg.PackageManager == "beta-one" || len(cfg.PackageManagers) != 0 {
				t.Errorf("a refused switch must not be written: %+v", cfg)
			}
		})
	}
}

func TestConfigSetPackageManagersList(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "set", "package_managers", "alpha-one, alpha-two"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	cfg, _ := readConfig(t, h.project)
	if !reflect.DeepEqual(cfg.PackageManagers, []string{"alpha-one", "alpha-two"}) {
		t.Errorf("config = %+v", cfg)
	}
	if code := h.run("config", "set", "package_managers", "alpha-one,nope"); code != core.ExitUsage {
		t.Errorf("unknown manager: exit %d", code)
	}
}

func TestDoctorReportsSeveralManagersPresent(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "1"})
	h.mark(t, "alpha-two")
	h.writeProjectConfig(t, h.project, "package_manager = \"alpha-one\"\n")
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if out := h.stdout.String(); !strings.Contains(out, "alpha-one, alpha-two all match") || !strings.Contains(out, "upc config set pm") {
		t.Errorf("stdout = %q", out)
	}
}

func TestDoctorReportsManagerOtherThanConfigured(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "1"})
	h.writeProjectConfig(t, h.project, "package_manager = \"alpha-two\"\n")
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if out := h.stdout.String(); !strings.Contains(out, "alpha-one's files are present but config names alpha-two") {
		t.Errorf("stdout = %q", out)
	}
}

func TestDoctorAcceptsConfiguredSetOfManagers(t *testing.T) {
	h := newHarness(t)
	h.useRunner(&probeRunner{version: "1"})
	h.mark(t, "alpha-two")
	h.writeProjectConfig(t, h.project, "package_manager = \"alpha-one\"\npackage_managers = [\"alpha-one\", \"alpha-two\"]\n")
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if out := h.stdout.String(); strings.Contains(out, "all match") || strings.Contains(out, "files are present") {
		t.Errorf("a declared set is not drift: %q", out)
	}
}
