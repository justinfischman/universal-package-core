package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestWorkspaceListShowsMembersAndAppsInStartOrder(t *testing.T) {
	h := newHarness(t)
	// web is declared first alphabetically after api, but consumes it; the
	// listing follows start order, not name order.
	h.twoApps(t, `
[workspace]
members = ["apps/api", "apps/web"]
[apps.aaa-web]
path = "apps/web"
port = 3000
consumes = ["zzz-api"]
[apps.zzz-api]
path = "apps/api"
port = 8000
provides = ["http"]
`)
	if code := h.run("workspace", "list"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	out := h.stdout.String()
	for _, want := range []string{"members:", "apps/api", "apps/web", "1. zzz-api", "2. aaa-web", "consumes zzz-api"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestWorkspaceListJSON(t *testing.T) {
	h := newHarness(t)
	h.twoApps(t, "[workspace]\nmembers = [\"apps/api\", \"apps/web\"]\n"+twoAppConfig)
	if code := h.run("--json", "workspace", "list"); code != core.ExitOK {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}
	var got struct {
		Data workspaceListing `json:"data"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, h.stdout.String())
	}
	if want := []string{"apps/api", "apps/web"}; !reflect.DeepEqual(got.Data.Members, want) {
		t.Errorf("members = %v, want %v", got.Data.Members, want)
	}
	var order []string
	for _, a := range got.Data.Apps {
		order = append(order, a.Name)
	}
	if want := []string{"api", "web"}; !reflect.DeepEqual(order, want) {
		t.Errorf("apps = %v, want %v", order, want)
	}
}

func TestWorkspaceListWithoutAppsFails(t *testing.T) {
	h := newHarness(t)
	if code := h.run("workspace", "list"); code == core.ExitOK {
		t.Fatal("want failure when no apps are configured")
	}
}
