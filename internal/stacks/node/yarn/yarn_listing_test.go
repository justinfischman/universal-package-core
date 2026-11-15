package yarn

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "yarn", Args: strings.Fields(cmd)[1:]}
	resp := execx.Response{Output: execx.Output{Stdout: out, ExitCode: code}}
	if code != 0 {
		resp.Err = &execx.ExitError{Cmd: c, Code: code}
	}
	return &execx.FakeRunner{Responses: map[string]execx.Response{c.String(): resp}}
}

func TestListParsesRecordedOutput(t *testing.T) {
	cases := []struct {
		fixture string
		files   map[string]string
		want    []core.Package
		summary string
	}{
		// is-number is transitive and @types/node is scoped: only declared names, in name order.
		{"ls_deps.ndjson", projectFiles, []core.Package{{Name: "@types/node", Version: "22.0.0"}, {Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}},
			"@types/node 22.0.0\nis-even 0.1.0\nis-odd 2.0.0"},
		{"ls_empty.ndjson", bareFiles, []core.Package{}, "no dependencies"},
		// declared but not installed: no version
		{"ls_empty.ndjson", projectFiles, []core.Package{{Name: "@types/node"}, {Name: "is-even"}, {Name: "is-odd"}}, "@types/node\nis-even\nis-odd"},
	}
	for _, c := range cases {
		t.Run(c.fixture+"/"+c.summary, func(t *testing.T) {
			fake := scripted("yarn list --depth=0 --json", testdata(t, c.fixture), 0)
			res, err := call(t, core.CapList, core.Request{Dir: fixture(t, c.files)}, fake)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
			if fake.Calls[len(fake.Calls)-1].Stream {
				t.Error("parsed output must be captured, not streamed")
			}
		})
	}
}

func TestListReportsFailures(t *testing.T) {
	req := core.Request{Dir: fixture(t, projectFiles)}
	_, err := call(t, core.CapList, req, scripted("yarn list --depth=0 --json", testdata(t, "ls_error.ndjson"), 1))
	if err == nil || !strings.Contains(err.Error(), "Expected property name") {
		t.Errorf("err = %v, want yarn's message", err)
	}
	_, err = call(t, core.CapList, req, scripted("yarn list --depth=0 --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	_, err = call(t, core.CapList, req, scripted("yarn list --depth=0 --json", "{not json", 0))
	if err == nil || !strings.Contains(err.Error(), "parse yarn list output") {
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
		// yarn outdated exits 1 when it finds outdated packages: that is a result.
		{"outdated.ndjson", 1, []core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}},
			"is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1"},
		{"outdated_empty.ndjson", 0, []core.Package{}, "all dependencies are up to date"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("yarn outdated --json", testdata(t, c.fixture), c.code))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Data, c.want) || res.Summary != c.summary {
				t.Errorf("result = %+v, want %+v / %q", res, c.want, c.summary)
			}
		})
	}
}

func TestOutdatedReportsFailures(t *testing.T) {
	req := core.Request{Dir: fixture(t, projectFiles)}
	_, err := call(t, core.CapOutdated, req, scripted("yarn outdated --json", testdata(t, "outdated_error.ndjson"), 1))
	if err == nil || !strings.Contains(err.Error(), `Couldn't find package "nope"`) {
		t.Errorf("err = %v, want yarn's message", err)
	}
	_, err = call(t, core.CapOutdated, req, scripted("yarn outdated --json", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("exit with no output: err = %v, want the ExitError", err)
	}
	for _, bad := range []string{`{"type":"table","data":{"head":["Name"],"body":[]}}`, `{"type":"table","data":{"head":["Package","Current","Latest"],"body":[["a"]]}}`} {
		if _, err = call(t, core.CapOutdated, req, scripted("yarn outdated --json", bad, 0)); err == nil {
			t.Errorf("unexpected shape %s must be an error", bad)
		}
	}
}
