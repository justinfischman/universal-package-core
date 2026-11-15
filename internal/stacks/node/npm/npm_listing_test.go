package npm

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "npm", Args: strings.Fields(cmd)[1:]}
	resp := execx.Response{Output: execx.Output{Stdout: out, ExitCode: code}}
	if code != 0 {
		resp.Err = &execx.ExitError{Cmd: c, Code: code}
	}
	return &execx.FakeRunner{Responses: map[string]execx.Response{c.String(): resp}}
}

func TestListParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		code    int
		want    []core.Package
		summary string
	}{
		{"ls_deps.json", 0, []core.Package{{Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}}, "is-even 0.1.0\nis-odd 2.0.0"},
		{"ls_empty.json", 0, []core.Package{}, "no dependencies"},
		{"ls_missing.json", 1, []core.Package{{Name: "is-odd", Version: "2.0.0"}, {Name: "zod"}}, "is-odd 2.0.0\nzod"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("npm ls --depth=0 --json", testdata(t, c.fixture), c.code)
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

func TestListReportsNpmErrors(t *testing.T) {
	_, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", testdata(t, "ls_error.json"), 1))
	if err == nil || !strings.Contains(err.Error(), "EJSONPARSE") {
		t.Errorf("err = %v, want the npm error code", err)
	}
	_, err = call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm ls --depth=0 --json", "{not json", 0))
	if err == nil || !strings.Contains(err.Error(), "parse npm ls output") {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestOutdatedParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		code    int
		want    []core.Package
		summary string
	}{
		// npm outdated exits 1 when it finds outdated packages: that is a result, not a failure.
		{"outdated.json", 1, []core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}},
			"is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1"},
		{"outdated_empty.json", 0, []core.Package{}, "all dependencies are up to date"},
		{"outdated_multi.json", 1, []core.Package{{Name: "left-pad", Version: "1.0.0", Latest: "1.3.0"}, {Name: "not-installed", Latest: "2.0.0"}},
			"left-pad 1.0.0 -> 1.3.0\nnot-installed  -> 2.0.0"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("npm outdated --json", testdata(t, c.fixture), c.code)
			res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
		})
	}
}

func TestOutdatedEmptyOutputMeansUpToDate(t *testing.T) {
	res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "", 0))
	if err != nil || res.Summary != "all dependencies are up to date" {
		t.Errorf("result = %+v, err = %v", res, err)
	}
}

func TestOutdatedReportsFailures(t *testing.T) {
	_, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", testdata(t, "outdated_error.json"), 1))
	if err == nil || !strings.Contains(err.Error(), "ENOTFOUND") {
		t.Errorf("err = %v, want the npm error code", err)
	}
	_, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("npm outdated --json", "[1]", 0))
	if err == nil {
		t.Error("unexpected shape must be an error")
	}
}
