package poetry

import (
	"errors"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestInitPackage(t *testing.T) {
	fake := &execx.FakeRunner{}
	if _, err := call(t, core.CapInit, core.Request{Dir: t.TempDir(), ProjectType: "package", Name: "demo"}, fake); err != nil {
		t.Fatal(err)
	}
	if got := argv(fake); !reflect.DeepEqual(got, []string{"poetry new --src demo"}) {
		t.Errorf("ran %q", got)
	}
	for name, req := range map[string]core.Request{
		"no name":   {ProjectType: "package"},
		"flag-like": {ProjectType: "package", Name: "--x"},
		"path":      {ProjectType: "package", Name: "a/b"},
	} {
		req.Dir = t.TempDir()
		fake := &execx.FakeRunner{}
		if _, err := call(t, core.CapInit, req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
			t.Errorf("%s: err = %v, ran %q", name, err, argv(fake))
		}
	}
}

func TestRejectsBadInput(t *testing.T) {
	dir := fixture(t, projectFiles)
	cases := map[string]struct {
		cap core.Capability
		req core.Request
	}{
		"add no packages":   {core.CapAdd, core.Request{Dir: dir}},
		"add flag-like pkg": {core.CapAdd, core.Request{Dir: dir, Packages: []string{"--source=evil"}}},
		"add flag group":    {core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Group: "--bad"}},
		"add script":        {core.CapAdd, core.Request{Dir: dir, Packages: []string{"x"}, Script: "a.py"}},
		"add empty dir":     {core.CapAdd, core.Request{Dir: t.TempDir(), Packages: []string{"x"}}},
		"install flag":      {core.CapInstall, core.Request{Dir: dir, Group: "-x"}},
		"update flag-like":  {core.CapUpdate, core.Request{Dir: dir, Packages: []string{"-x"}}},
		"init script":       {core.CapInit, core.Request{Dir: dir, ProjectType: "script", Script: "a"}},
		"init unknown":      {core.CapInit, core.Request{Dir: dir, ProjectType: "spaceship"}},
		"list empty dir":    {core.CapList, core.Request{Dir: t.TempDir()}},
		"test empty dir":    {core.CapTest, core.Request{Dir: t.TempDir()}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := call(t, c.cap, c.req, fake); !errors.Is(err, core.ErrInvalidRequest) || len(fake.Calls) != 0 {
				t.Errorf("err = %v, ran %v", err, argv(fake))
			}
		})
	}
}

func TestListAndOutdatedParseOutput(t *testing.T) {
	dir := fixture(t, projectFiles)
	respond := func(args, out string) *execx.FakeRunner {
		return &execx.FakeRunner{Responses: map[string]execx.Response{args: {Output: execx.Output{Stdout: out}}}}
	}

	res, err := call(t, core.CapList, core.Request{Dir: dir},
		respond("poetry show --top-level", "requests 2.31.0 Python HTTP for Humans.\nrich    15.0.0  Render rich text\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Name: "requests", Version: "2.31.0"}, {Name: "rich", Version: "15.0.0"}}
	if !reflect.DeepEqual(res.Data, want) || res.Summary != "requests 2.31.0\nrich 15.0.0" {
		t.Errorf("list = %+v", res)
	}

	res, err = call(t, core.CapOutdated, core.Request{Dir: dir},
		respond("poetry show --top-level --outdated", "requests 2.31.0 2.34.2 Python HTTP\nrich (!) 15.0.0 15.0.0 text\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Data, []core.Package{{Name: "requests", Version: "2.31.0", Latest: "2.34.2"}}) ||
		res.Summary != "requests 2.31.0 -> 2.34.2" {
		t.Errorf("outdated = %+v", res)
	}
}

func TestListEmptyIsReported(t *testing.T) {
	res, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, &execx.FakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary != "no dependencies" || len(res.Data.([]core.Package)) != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestNonZeroExitIsReturnedAsExitError(t *testing.T) {
	cmd := execx.Cmd{Name: "poetry", Args: []string{"add", "nope"}}
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		cmd.String(): {Err: &execx.ExitError{Cmd: cmd, Code: 1, Stderr: "package not found"}},
	}}
	_, err := call(t, core.CapAdd, core.Request{Dir: fixture(t, projectFiles), Packages: []string{"nope"}}, fake)
	var exit *execx.ExitError
	if !errors.As(err, &exit) || exit.Code != 1 || core.ExitCode(err) != core.ExitFailure {
		t.Fatalf("err = %v, want ExitError code 1", err)
	}
}
