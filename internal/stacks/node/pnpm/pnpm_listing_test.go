package pnpm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "pnpm", Args: strings.Fields(cmd)[1:]}
	resp := execx.Response{Output: execx.Output{Stdout: out, ExitCode: code}}
	if code != 0 {
		resp.Err = &execx.ExitError{Cmd: c, Code: code}
	}
	return &execx.FakeRunner{Responses: map[string]execx.Response{c.String(): resp}}
}

func TestListParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		want    []core.Package
		summary string
	}{
		{"ls_deps.json", []core.Package{{Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}}, "is-even 0.1.0\nis-odd 2.0.0"},
		{"ls_empty.json", []core.Package{}, "no dependencies"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("pnpm ls --depth=0 --json", testdata(t, c.fixture), 0)
			res, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
			if fake.Calls[0].Stream {
				t.Error("parsed output must be captured, not streamed")
			}
		})
	}
}

func TestListReportsFailures(t *testing.T) {
	dir := fixture(t, projectFiles)
	_, err := call(t, core.CapList, core.Request{Dir: dir}, scripted("pnpm ls --depth=0 --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapList, core.Request{Dir: dir}, scripted("pnpm ls --depth=0 --json", "{not json", 0))
	if err == nil || !strings.Contains(err.Error(), "parse pnpm ls output") {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestOutdatedParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		code    int
		want    []core.Package
		summary string
	}{
		// pnpm outdated exits 1 when it finds outdated packages: that is a result, not a failure.
		{"outdated", testdata(t, "outdated.json"), 1,
			[]core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}},
			"is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1"},
		{"empty object", "{}", 0, []core.Package{}, "all dependencies are up to date"},
		{"no output", "", 0, []core.Package{}, "all dependencies are up to date"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := scripted("pnpm outdated --format json", c.out, c.code)
			res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Data.([]core.Package)) != len(c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
		})
	}
}

func TestOutdatedReportsFailures(t *testing.T) {
	dir := fixture(t, projectFiles)
	_, err := call(t, core.CapOutdated, core.Request{Dir: dir}, scripted("pnpm outdated --format json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapOutdated, core.Request{Dir: dir}, scripted("pnpm outdated --format json", "[1]", 0))
	if err == nil || !strings.Contains(err.Error(), "parse pnpm outdated output") {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestListMergesProjectsOnce(t *testing.T) {
	pkgs, err := ParseList(`[{"dependencies":{"a":{"version":"1.0.0"}}},{"dependencies":{"a":{"version":"1.0.0"}},"optionalDependencies":{"b":{}}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []core.Package{{Name: "a", Version: "1.0.0"}, {Name: "b"}}; !reflect.DeepEqual(pkgs, want) {
		t.Errorf("packages = %+v, want %+v", pkgs, want)
	}
}
