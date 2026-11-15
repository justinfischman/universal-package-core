package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

const twoAppConfig = `
[apps.api]
path = "apps/api"
port = 8000
url = "http://localhost:${apps.api.port}"
provides = ["http"]

[apps.web]
path = "apps/web"
port = 3000
consumes = ["api"]
[apps.web.env]
API_URL = "${apps.api.url}/v1"
`

func (h *harness) twoApps(t *testing.T, config string) {
	t.Helper()
	h.writeProjectConfig(t, h.project, config)
	for _, app := range []string{"api", "web"} {
		dir := filepath.Join(h.project, "apps", app)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "alpha-one.marker"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUpAllStartsInOrderAndDownReverses(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, twoAppConfig)
	if code := h.run("up", "--all"); code != core.ExitOK {
		t.Fatalf("up exit %d: %s", code, h.stderr)
	}
	if code := h.run("down", "--all"); code != core.ExitOK {
		t.Fatalf("down exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	var got []string
	for i, r := range pm.Requests {
		got = append(got, string(pm.Calls[i])+" "+r.Member)
	}
	if want := []string{"up api", "up web", "down web", "down api"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
	web := pm.Requests[1]
	if !reflect.DeepEqual(web.Env, []string{"API_URL=http://localhost:8000/v1"}) || web.Dir != filepath.Join(h.project, "apps", "web") {
		t.Errorf("web request = %+v", web)
	}
}

func TestUpAppStartsOnlyThatApp(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, twoAppConfig)
	if code := h.run("up", "--app", "api"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	pm := h.pm(t, "alpha-one")
	if len(pm.Requests) != 1 || pm.Requests[0].Member != "api" {
		t.Fatalf("requests = %+v", pm.Requests)
	}
}

func TestUpAppsRejectsBadCombinations(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, twoAppConfig)
	for _, args := range [][]string{
		{"up", "--all", "--app", "api"},
		{"up", "--all", "-w", "api"},
		{"up", "--app", "ghost"},
		{"up", "--all", "--", "--port", "1"},
	} {
		if code := h.run(args...); code == core.ExitOK {
			t.Errorf("%v: want failure", args)
		}
	}
}

func TestDoctorReportsCrossAppFindings(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, `
[apps.api]
path = "apps/api"
port = 3000
[apps.web]
path = "apps/web"
port = 3000
consumes = ["api"]
[apps.web.env]
API_URL = "${apps.api.url}"
`)
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	for _, kind := range []string{"port-collision", "consumer-without-provider", "unresolved-variable"} {
		if !strings.Contains(out, kind) {
			t.Errorf("doctor output lacks %s:\n%s", kind, out)
		}
	}
}
