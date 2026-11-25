package pip

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitCreatesEmptyRequirementsOnce(t *testing.T) {
	m := newMem(nil)
	if _, err := call(t, m, core.CapInit, core.Request{}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if body, ok := m.files[reqPath]; !ok || body != "" {
		t.Fatalf("requirements.txt = %q, %v", body, ok)
	}
	m.files[reqPath] = "rich==1\n"
	if _, err := call(t, m, core.CapInit, core.Request{}, &execx.FakeRunner{}); err != nil {
		t.Fatal(err)
	}
	if m.files[reqPath] != "rich==1\n" {
		t.Error("init must not overwrite an existing requirements.txt")
	}
}

func TestInitScript(t *testing.T) {
	m := newMem(nil)
	fake := &execx.FakeRunner{}
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "script", Script: "hello"}, fake); err != nil {
		t.Fatal(err)
	}
	if body := m.files[filepath.Join(proj, "hello.py")]; !strings.HasPrefix(body, "# /// script\n") || len(fake.Calls) != 0 {
		t.Errorf("hello.py = %q, ran %q", body, argv(fake))
	}
	for name, req := range map[string]core.Request{
		"exists":       {ProjectType: "script", Script: "hello"},
		"bad name":     {ProjectType: "script", Script: "--x"},
		"path in name": {ProjectType: "script", Script: "a/b"},
		"other type":   {ProjectType: "spaceship"},
	} {
		if _, err := call(t, m, core.CapInit, req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestInitPackage(t *testing.T) {
	m := newMem(nil)
	fake := &execx.FakeRunner{}
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "My-Lib"}, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("ran %q; pip package init only writes files", argv(fake))
	}
	for _, f := range []string{"pyproject.toml", "README.md", "requirements.txt", "src/my_lib/__init__.py"} {
		if _, ok := m.files[filepath.Join(proj, "My-Lib", f)]; !ok {
			t.Errorf("missing %s; have %v", f, m.files)
		}
	}
	if body := m.files[filepath.Join(proj, "My-Lib", "pyproject.toml")]; !strings.Contains(body, `name = "My-Lib"`) {
		t.Errorf("pyproject.toml = %q", body)
	}
}

func TestInitPackageDryRunWritesNothing(t *testing.T) {
	m := newMem(nil)
	res, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "demo", DryRun: true}, &execx.FakeRunner{})
	if err != nil || len(m.files) != 0 || !strings.HasPrefix(res.Summary, "dry run") {
		t.Errorf("res = %+v, err = %v, files = %v", res, err, m.files)
	}
}

func TestInitPackageRejectsBadInput(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		req   core.Request
	}{
		"no name":      {nil, core.Request{ProjectType: "package"}},
		"flag-like":    {nil, core.Request{ProjectType: "package", Name: "--x"}},
		"path in name": {nil, core.Request{ProjectType: "package", Name: "a/b"}},
		"exists":       {map[string]string{"demo": ""}, core.Request{ProjectType: "package", Name: "demo"}},
	} {
		if _, err := call(t, newMem(tc.files), core.CapInit, tc.req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestInitPackageFailureLeavesNothing(t *testing.T) {
	m := newMem(nil)
	m.failWrite = errors.New("disk full")
	if _, err := call(t, m, core.CapInit, core.Request{ProjectType: "package", Name: "demo"}, &execx.FakeRunner{}); err == nil {
		t.Fatal("want an error")
	}
	if len(m.files) != 0 {
		t.Errorf("files = %v", m.files)
	}
}
