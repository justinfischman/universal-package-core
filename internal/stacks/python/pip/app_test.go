package pip

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitApp(t *testing.T) {
	m := newMem(nil)
	fake := &execx.FakeRunner{}
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "app", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("ran %q; pip app init only writes files", argv(fake))
	}
	if _, ok := m.files[filepath.Join(proj, "demo", "requirements.txt")]; !ok {
		t.Errorf("missing requirements.txt; have %v", m.files)
	}
	body := m.files[filepath.Join(proj, "demo", "pyproject.toml")]
	for _, want := range []string{`name = "demo"`, "[tool.upc.scripts]", `dev = "python server.py"`, `start = "python server.py"`} {
		if !strings.Contains(body, want) {
			t.Errorf("pyproject.toml lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, "[build-system]") {
		t.Errorf("an app is not a distributable package:\n%s", body)
	}
}

func TestInitAppDryRunWritesNothing(t *testing.T) {
	m := newMem(nil)
	res, err := call(t, m, core.CapInit, core.Request{ProjectType: "app", Name: "demo", DryRun: true}, &execx.FakeRunner{})
	if err != nil || len(m.files) != 0 || !strings.HasPrefix(res.Summary, "dry run") {
		t.Errorf("res = %+v, err = %v, files = %v", res, err, m.files)
	}
}

func TestInitAppRejectsBadInputAndCleansUp(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		req   core.Request
	}{
		"no name":   {nil, core.Request{ProjectType: "app"}},
		"flag-like": {nil, core.Request{ProjectType: "app", Name: "--x"}},
		"exists":    {map[string]string{"demo": ""}, core.Request{ProjectType: "app", Name: "demo"}},
	} {
		if _, err := call(t, newMem(tc.files), core.CapInit, tc.req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	m := newMem(nil)
	m.failWrite = errors.New("disk full")
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "app", Name: "demo"}, &execx.FakeRunner{}); err == nil || len(m.files) != 0 {
		t.Errorf("err = %v, files = %v", err, m.files)
	}
}

// TestInitAppPassesTheKindToTheScripts requires req.Kind to reach the app
// scripts, so `--kind cli` and `--kind library` run their own entry file.
func TestInitAppPassesTheKindToTheScripts(t *testing.T) {
	for kind, command := range map[string]string{"cli": "python cli.py", "library": "python demo.py"} {
		m := newMem(nil)
		if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "app", Name: "demo", Kind: kind}, &execx.FakeRunner{}); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		body := m.files[filepath.Join(proj, "demo", "pyproject.toml")]
		want := "dev = \"" + command + "\"\nstart = \"" + command + "\"\n"
		if !strings.Contains(body, want) {
			t.Errorf("%s pyproject.toml lacks %q:\n%s", kind, want, body)
		}
	}
}
