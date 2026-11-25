package bun

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func scripted(cmd string, out string, code int) *execx.FakeRunner {
	c := execx.Cmd{Name: "bun", Args: strings.Fields(cmd)[1:]}
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
		{"pm_ls.txt", []core.Package{{Name: "is-even", Version: "0.1.0"}, {Name: "is-odd", Version: "2.0.0"}}, "is-even 0.1.0\nis-odd 2.0.0"},
		{"pm_ls_empty.txt", []core.Package{}, "no dependencies"},
	}
	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			fake := scripted("bun pm ls", testdata(t, c.fixture), 0)
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

func TestListSkipsTransitiveAndKeepsScopes(t *testing.T) {
	text := "/w node_modules (5 installed)\n├── @types/node@22.1.0\n├── is-even@0.1.0 (dev)\n│   └── is-number@1.1.2\n└── zod@3.0.0\n"
	want := []core.Package{{Name: "@types/node", Version: "22.1.0"}, {Name: "is-even", Version: "0.1.0"}, {Name: "zod", Version: "3.0.0"}}
	if got := ParseList(text); !reflect.DeepEqual(got, want) {
		t.Errorf("packages = %+v, want %+v", got, want)
	}
}

func TestListFailureWithoutOutputIsReported(t *testing.T) {
	// bun reports a missing lock file on stderr and exits 1.
	_, err := call(t, core.CapList, core.Request{Dir: fixture(t, projectFiles)}, scripted("bun pm ls", "", 1))
	var exit *execx.ExitError
	if !errors.As(err, &exit) {
		t.Errorf("err = %v, want the ExitError", err)
	}
}

func TestOutdatedParsesRecordedOutput(t *testing.T) {
	fake := scripted("bun outdated", testdata(t, "outdated.txt"), 0)
	res, err := call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, fake)
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Name: "is-even", Version: "0.1.0", Latest: "1.0.0"}, {Name: "is-odd", Version: "2.0.0", Latest: "3.0.1"}}
	if !reflect.DeepEqual(res.Data, want) || res.Summary != "is-even 0.1.0 -> 1.0.0\nis-odd 2.0.0 -> 3.0.1" {
		t.Errorf("result = %+v, want %+v", res, want)
	}
	res, err = call(t, core.CapOutdated, core.Request{Dir: fixture(t, projectFiles)}, scripted("bun outdated", "bun outdated v1.4.2\n", 0))
	if err != nil || res.Summary != "all dependencies are up to date" {
		t.Errorf("up to date: result = %+v, err = %v", res, err)
	}
}

func TestLockedVersionsSkipsNestedCopies(t *testing.T) {
	want := map[string][]string{"is-even": {"0.1.0"}, "is-number": {"4.0.0"}, "is-odd": {"2.0.0"}}
	if got := lockedVersions(testdata(t, "bun.lock")); !reflect.DeepEqual(got, want) {
		t.Errorf("locked = %v, want %v", got, want)
	}
}

func TestLockDriftReportsStaleInstall(t *testing.T) {
	files := fstest.MapFS{
		"package.json":                      {Data: []byte(`{"dependencies":{"is-even":"0.1.0"}}`)},
		"bun.lock":                          {Data: []byte(testdata(t, "bun.lock"))},
		"node_modules/is-even/package.json": {Data: []byte(`{"version":"0.1.2"}`)},
	}
	if findings := lockDrift(files); len(findings) == 0 {
		t.Error("want a drift finding for an installed version the lock does not hold")
	}
	files["node_modules/is-even/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"0.1.0"}`)}
	if findings := lockDrift(files); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLegacyBinaryLockCountsAsLocked(t *testing.T) {
	files := fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "bun.lockb": {}}
	b := newBun(t)
	if got := b.DetectRank(files); got != rankLockfile {
		t.Errorf("rank = %d, want %d", got, rankLockfile)
	}
	if findings := b.Diagnose(files); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}
