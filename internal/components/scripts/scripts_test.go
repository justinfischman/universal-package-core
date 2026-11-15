package scripts

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
)

type tableReader struct {
	table Table
	err   error
}

func (r tableReader) ReadScripts(fs.FS) (Table, error) { return r.table, r.err }

func TestSplit(t *testing.T) {
	cases := map[string][]string{
		"a b":            {"a", "b"},
		`a "b c" d`:      {"a", "b c", "d"},
		`a 'b "c"' d`:    {"a", `b "c"`, "d"},
		"  a   b  ":      {"a", "b"},
		`a ""`:           {"a", ""},
		"tab\tseparated": {"tab", "separated"},
	}
	for in, want := range cases {
		got, err := Split(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", `a "b`} {
		if _, err := Split(bad); err == nil {
			t.Errorf("Split(%q) should fail", bad)
		}
	}
}

func TestResolveOrder(t *testing.T) {
	fsys := fstest.MapFS{"serve.py": {Data: []byte("x")}, "dir/x": {}}
	r := tableReader{table: Table{
		Commands:    map[string]string{"test": `pytest -q "a b"`, "both": "echo upc"},
		EntryPoints: map[string]string{"serve": "m:main", "both": "m:one", "cli": "m:cli"},
	}}
	cases := []struct {
		name string
		want Target
	}{
		{"serve.py", Target{Kind: KindFile, Name: "serve.py"}},
		{"serve", Target{Kind: KindEntryPoint, Name: "serve"}},
		{"test", Target{Kind: KindCommand, Name: "test", Words: []string{"pytest", "-q", "a b"}}},
		{"both", Target{Kind: KindCommand, Name: "both", Words: []string{"echo", "upc"}}},
		{"cli", Target{Kind: KindEntryPoint, Name: "cli"}},
	}
	for _, c := range cases {
		got, err := Resolve(fsys, c.name, r)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Resolve(%q) = %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
}

func TestResolveRejects(t *testing.T) {
	fsys := fstest.MapFS{"dir/x": {}}
	r := tableReader{table: Table{Commands: map[string]string{"ok": "echo", "bad": `echo "`}}}
	for _, name := range []string{"", "--help", "missing", "dir", "bad", "../x"} {
		if _, err := Resolve(fsys, name, r); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("Resolve(%q) err = %v, want invalid request", name, err)
		}
	}
	_, err := Resolve(fsys, "missing", r)
	if err == nil || !strings.Contains(err.Error(), "available: bad, ok") {
		t.Errorf("error should list the available scripts: %v", err)
	}
}

func TestResolveMissingManifestMeansNoScripts(t *testing.T) {
	fsys := fstest.MapFS{"run.py": {}}
	r := tableReader{err: fs.ErrNotExist}
	if got, err := Resolve(fsys, "run.py", r); err != nil || got.Kind != KindFile {
		t.Errorf("file with no manifest: %+v, %v", got, err)
	}
	if _, err := Resolve(fsys, "x", r); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v", err)
	}
	boom := errors.New("boom")
	if _, err := Resolve(fsys, "x", tableReader{err: boom}); !errors.Is(err, boom) {
		t.Errorf("reader errors must surface: %v", err)
	}
}

func TestResolveFirstOrderAndMiss(t *testing.T) {
	r := tableReader{table: Table{
		Commands:    map[string]string{"start": "serve --port 1", "dev": "flask run"},
		EntryPoints: map[string]string{"up": "demo-up"},
	}}
	got, err := ResolveFirst(nil, UpNames, r)
	if err != nil || got.Kind != KindEntryPoint || got.Name != "up" {
		t.Fatalf("up = %+v, %v", got, err)
	}
	delete(r.table.EntryPoints, "up")
	got, err = ResolveFirst(nil, UpNames, r)
	if err != nil || got.Name != "dev" || len(got.Words) != 2 {
		t.Fatalf("dev = %+v, %v", got, err)
	}
	delete(r.table.Commands, "dev")
	delete(r.table.Commands, "start")
	if _, err := ResolveFirst(nil, UpNames, r); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("miss = %v, want ErrInvalidRequest", err)
	}
}
