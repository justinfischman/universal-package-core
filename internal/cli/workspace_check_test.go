package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

const contractAppConfig = `
[apps.api]
path = "apps/api"
port = 8000
provides = ["http"]
contract = "openapi.yaml"
[apps.web]
path = "apps/web"
port = 3000
consumes = ["api"]
`

const checkSpecV1 = "openapi: 3.0.0\npaths:\n  /pets:\n    get:\n      responses:\n        \"200\": {description: ok}\n"

const checkSpecV2 = checkSpecV1 + "    post:\n      responses:\n        \"201\": {description: created}\n"

// contractHarness is a two-app project whose api app declares a contract.
func contractHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.twoApps(t, contractAppConfig)
	spec := filepath.Join(h.project, "apps", "api", "openapi.yaml")
	if err := os.WriteFile(spec, []byte(checkSpecV1), 0o644); err != nil {
		t.Fatal(err)
	}
	return h, spec
}

func TestWorkspaceCheckFailsWhenLockMissingAndWritesNothing(t *testing.T) {
	h, _ := contractHarness(t)
	if code := h.run("workspace", "check"); code == core.ExitOK {
		t.Fatalf("want failure for a missing lock:\n%s", h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "api: missing") {
		t.Errorf("output = %s", h.stdout.String())
	}
	lock := filepath.Join(h.project, ".universal-package-core", "contracts", "api.lock")
	if _, err := os.Stat(lock); err == nil {
		t.Fatal("check wrote a lock without --update")
	}
}

func TestWorkspaceCheckUpdateThenPassThenChangedFails(t *testing.T) {
	h, spec := contractHarness(t)
	if code := h.run("workspace", "check", "--update"); code != core.ExitOK {
		t.Fatalf("update exit %d: %s", code, h.stderr)
	}
	if out := h.stdout.String(); !strings.Contains(out, "api: missing -> lock written") {
		t.Errorf("update output = %s", out)
	}
	h.stdout.Reset()
	if code := h.run("workspace", "check"); code != core.ExitOK {
		t.Fatalf("check after update exit %d: %s", code, h.stdout.String())
	}
	if !strings.Contains(h.stdout.String(), "api: ok") {
		t.Errorf("output = %s", h.stdout.String())
	}
	if err := os.WriteFile(spec, []byte(checkSpecV2), 0o644); err != nil {
		t.Fatal(err)
	}
	h.stdout.Reset()
	if code := h.run("workspace", "check"); code == core.ExitOK {
		t.Fatal("want failure for a changed contract")
	}
	if out := h.stdout.String(); !strings.Contains(out, "api: changed") || !strings.Contains(out, "POST /pets") {
		t.Errorf("output lacks the diff summary:\n%s", out)
	}
}

func TestWorkspaceCheckJSON(t *testing.T) {
	h, _ := contractHarness(t)
	if code := h.run("--json", "workspace", "check", "--update"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	var got struct {
		Data contractReport `json:"data"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, h.stdout.String())
	}
	if len(got.Data.Apps) != 1 || got.Data.Apps[0].App != "api" || !got.Data.Apps[0].Wrote || !got.Data.Updated {
		t.Errorf("data = %+v", got.Data)
	}
}

func TestWorkspaceCheckDryRunUpdateWritesNothing(t *testing.T) {
	h, _ := contractHarness(t)
	if code := h.run("--dry-run", "workspace", "check", "--update"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(filepath.Join(h.project, ".universal-package-core", "contracts", "api.lock")); err == nil {
		t.Fatal("dry run wrote a lock")
	}
}

func TestWorkspaceCheckWithoutContractsFails(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, twoAppConfig)
	if code := h.run("workspace", "check"); code == core.ExitOK {
		t.Fatal("want failure when no app declares a contract")
	}
}

func TestDoctorReportsContractMissingThenChanged(t *testing.T) {
	h, spec := contractHarness(t)
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	if !strings.Contains(h.stdout.String(), "contract-missing") {
		t.Errorf("doctor lacks contract-missing:\n%s", h.stdout.String())
	}
	if code := h.run("workspace", "check", "--update"); code != core.ExitOK {
		t.Fatalf("update exit %d", code)
	}
	if err := os.WriteFile(spec, []byte(checkSpecV2), 0o644); err != nil {
		t.Fatal(err)
	}
	h.stdout.Reset()
	if code := h.run("doctor"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	if !strings.Contains(out, "contract-changed") || strings.Contains(out, "contract-missing") {
		t.Errorf("doctor output:\n%s", out)
	}
}
