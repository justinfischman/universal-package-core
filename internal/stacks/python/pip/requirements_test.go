package pip

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var reqPath = filepath.Join(proj, "requirements.txt")

func freezeOut(s string) *execx.FakeRunner {
	return respond(map[string]string{py + " -m pip freeze": s})
}

func TestAddWritesExactPins(t *testing.T) {
	cases := []struct {
		name     string
		existing map[string]string
		req      core.Request
		freeze   string
		want     string
	}{
		{"new file", venv, core.Request{Packages: []string{"rich"}}, "rich==13.7.0\n", "rich==13.7.0\n"},
		{"appends keeps comments and options",
			with(venv, map[string]string{"requirements.txt": "# deps\n-r base.txt\nattrs==23.1\n\n"}),
			core.Request{Packages: []string{"rich"}}, "attrs==23.1\nrich==13.7.0\n", "# deps\n-r base.txt\nattrs==23.1\n\nrich==13.7.0\n"},
		{"range becomes pin", venv, core.Request{Packages: []string{"rich>=13"}}, "rich==13.7.0\n", "rich==13.7.0\n"},
		{"keeps extras and marker", venv, core.Request{Packages: []string{`Requests[security]>=2; python_version >= "3.9"`}},
			"requests==2.31.0\n", "Requests[security]==2.31.0; python_version >= \"3.9\"\n"},
		{"matches normalized names", with(venv, map[string]string{"requirements.txt": "typing_extensions==4.0\n"}),
			core.Request{Packages: []string{"Typing-Extensions"}}, "typing_extensions==4.9.0\n", "Typing-Extensions==4.9.0\n"},
		{"adds a missing trailing newline", with(venv, map[string]string{"requirements.txt": "attrs==1"}),
			core.Request{Packages: []string{"rich"}}, "rich==2\n", "attrs==1\nrich==2\n"},
		{"unnamed spec kept verbatim", venv, core.Request{Packages: []string{"git+https://example.com/x.git"}}, "", "git+https://example.com/x.git\n"},
		{"unknown to freeze keeps spec", venv, core.Request{Packages: []string{"rich>=13"}}, "", "rich>=13\n"},
		{"dev group", venv, core.Request{Packages: []string{"pytest"}, Group: "dev"}, "pytest==8.0.0\n", "pytest==8.0.0\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newMem(c.existing)
			if _, err := call(t, m, core.CapAdd, c.req, freezeOut(c.freeze)); err != nil {
				t.Fatal(err)
			}
			path := reqPath
			if c.req.Group == "dev" {
				path = filepath.Join(proj, "requirements-dev.txt")
			}
			if got := m.files[path]; got != c.want {
				t.Errorf("%s = %q, want %q", path, got, c.want)
			}
		})
	}
}

func TestAddIsIdempotent(t *testing.T) {
	m := newMem(venv)
	for i := range 3 {
		if _, err := call(t, m, core.CapAdd, core.Request{Packages: []string{"rich", "attrs"}}, freezeOut("attrs==23.1\nrich==13.7.0\n")); err != nil {
			t.Fatal(err)
		}
		if got := m.files[reqPath]; got != "rich==13.7.0\nattrs==23.1\n" {
			t.Fatalf("run %d: %q", i, got)
		}
	}
	if m.writes != 1 {
		t.Errorf("writes = %d, want 1 (unchanged content is not rewritten)", m.writes)
	}
}

func TestAddFailureLeavesFileUnchanged(t *testing.T) {
	original := "attrs==23.1\n"
	installFails := &execx.ExitError{Cmd: execx.Cmd{Name: py}, Code: 1, Stderr: "no such package"}
	cases := map[string]func(*memFS) *execx.FakeRunner{
		"install fails": func(*memFS) *execx.FakeRunner {
			return &execx.FakeRunner{Responses: map[string]execx.Response{py + " -m pip install rich": {Err: installFails}}}
		},
		"freeze fails": func(*memFS) *execx.FakeRunner {
			return &execx.FakeRunner{Responses: map[string]execx.Response{py + " -m pip freeze": {Err: installFails}}}
		},
		"write fails": func(m *memFS) *execx.FakeRunner {
			m.failWrite = errors.New("disk full")
			return freezeOut("rich==1\n")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			m := newMem(with(venv, map[string]string{"requirements.txt": original}))
			fake := setup(m)
			if _, err := call(t, m, core.CapAdd, core.Request{Packages: []string{"rich"}}, fake); err == nil {
				t.Fatal("want an error")
			}
			if got := m.files[reqPath]; got != original || m.writes != 0 {
				t.Errorf("file = %q, writes = %d; must be unchanged", got, m.writes)
			}
		})
	}
	// a failed first add must not create the file either
	m := newMem(venv)
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{py + " -m pip install rich": {Err: installFails}}}
	if _, err := call(t, m, core.CapAdd, core.Request{Packages: []string{"rich"}}, fake); err == nil {
		t.Fatal("want an error")
	}
	if _, ok := m.files[reqPath]; ok {
		t.Error("requirements.txt must not be created by a failed add")
	}
}

func TestRemoveEditsFile(t *testing.T) {
	m := newMem(with(venv, map[string]string{"requirements.txt": "# keep\nRich[jupyter]==13.0\nattrs==23.1\n"}))
	for range 2 { // second pass: removing what is gone is a no-op, not an error
		if _, err := call(t, m, core.CapRemove, core.Request{Packages: []string{"rich"}}, &execx.FakeRunner{}); err != nil {
			t.Fatal(err)
		}
		if got := m.files[reqPath]; got != "# keep\nattrs==23.1\n" {
			t.Fatalf("file = %q", got)
		}
	}
	if m.writes != 1 {
		t.Errorf("writes = %d, want 1", m.writes)
	}
	// uninstall failure leaves the file alone
	m = newMem(with(venv, map[string]string{"requirements.txt": "rich==1\n"}))
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{py + " -m pip uninstall --yes rich": {Err: errors.New("boom")}}}
	if _, err := call(t, m, core.CapRemove, core.Request{Packages: []string{"rich"}}, fake); err == nil || m.files[reqPath] != "rich==1\n" {
		t.Errorf("err = %v, file = %q", err, m.files[reqPath])
	}
}

func TestRemoveRejectsUnnamedSpecs(t *testing.T) {
	fake := &execx.FakeRunner{}
	_, err := call(t, newMem(venv), core.CapRemove, core.Request{Packages: []string{"git+https://x/y.git"}}, fake)
	if !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestUpdateRewritesOnlyPins(t *testing.T) {
	file := "# deps\nrich==13.0  # pinned\nattrs>=23\nrequests[security]==2.0; python_version>='3.9'\nflask==2.0,<3\n-r other.txt\n"
	freeze := "rich==14.0\nattrs==24.1\nrequests==2.31.0\nflask==2.3\n"

	m := newMem(with(venv, map[string]string{"requirements.txt": file}))
	if _, err := call(t, m, core.CapUpdate, core.Request{}, freezeOut(freeze)); err != nil {
		t.Fatal(err)
	}
	want := "# deps\nrich==14.0  # pinned\nattrs>=23\nrequests[security]==2.31.0; python_version>='3.9'\nflask==2.0,<3\n-r other.txt\n"
	if got := m.files[reqPath]; got != want {
		t.Errorf("update all:\n%q\nwant\n%q", got, want)
	}

	m = newMem(with(venv, map[string]string{"requirements.txt": file}))
	if _, err := call(t, m, core.CapUpdate, core.Request{Packages: []string{"Rich"}}, freezeOut(freeze)); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(file, "rich==13.0", "rich==14.0", 1)
	if got := m.files[reqPath]; got != want {
		t.Errorf("update named:\n%q\nwant\n%q", got, want)
	}
}

func TestUpdateAllNeedsTheFile(t *testing.T) {
	fake := &execx.FakeRunner{}
	if _, err := call(t, newMem(venv), core.CapUpdate, core.Request{}, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestInstallNeedsTheFile(t *testing.T) {
	fake := &execx.FakeRunner{}
	if _, err := call(t, newMem(venv), core.CapInstall, core.Request{}, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestInvalidInputIsRefusedBeforeRunning(t *testing.T) {
	for name, req := range map[string]core.Request{
		"no packages":  {},
		"flag package": {Packages: []string{"--index-url=x"}},
		"empty spec":   {Packages: []string{" "}},
		"bad group":    {Packages: []string{"rich"}, Group: "../x"},
		"flag group":   {Packages: []string{"rich"}, Group: "-r"},
	} {
		fake := &execx.FakeRunner{}
		if _, err := call(t, newMem(venv), core.CapAdd, req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %q", name, err, argv(fake))
		}
	}
}

func TestParseSpec(t *testing.T) {
	cases := []struct {
		in     string
		key    string
		extras string
		marker string
		ok     bool
	}{
		{"rich", "rich", "", "", true},
		{"Rich_Text>=1", "rich-text", "", "", true},
		{"a[x,y]==1; python_version<'3.9'  # c", "a", "[x,y]", "python_version<'3.9'", true},
		{"pkg @ https://x/y.whl", "pkg", "", "", true},
		{"git+https://x/y.git", "", "", "", false},
		{"./local", "", "", "", false},
		{"-r other.txt", "", "", "", false},
		{"# comment", "", "", "", false},
		{"", "", "", "", false},
	}
	for _, c := range cases {
		got, ok := parseSpec(c.in)
		if ok != c.ok || got.key != c.key || got.extras != c.extras || got.marker != c.marker {
			t.Errorf("parseSpec(%q) = %+v, %v", c.in, got, ok)
		}
	}
}

func TestRequirementsRoundTrip(t *testing.T) {
	for _, in := range []string{"", "a==1\n", "# c\n\nb==2\n-e .\n"} {
		if got := string(parseRequirements([]byte(in)).bytes()); got != in {
			t.Errorf("round trip %q -> %q", in, got)
		}
	}
	if got := string(parseRequirements([]byte("a==1")).bytes()); got != "a==1\n" {
		t.Errorf("missing newline normalized to %q", got)
	}
}

func TestParseFreeze(t *testing.T) {
	got := parseFreeze("Rich==13.0\ntyping_extensions==4.9\nlocal @ file:///x\n-e git+https://x#egg=y\n\n")
	want := map[string]string{"rich": "13.0", "typing-extensions": "4.9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("freeze = %v, want %v", got, want)
	}
}

func TestRequirementsFileNames(t *testing.T) {
	for group, want := range map[string]string{"": "requirements.txt", "dev": "requirements-dev.txt", "docs_2": "requirements-docs_2.txt"} {
		if got, err := requirementsFile(group); err != nil || got != want {
			t.Errorf("group %q = %q, %v", group, got, err)
		}
	}
	if _, err := requirementsFile("a/b"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v", err)
	}
}
