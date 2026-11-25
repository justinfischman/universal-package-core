package gomod

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/stacks/golang"
)

func run(t *testing.T, c core.Capability, req core.Request, fake *execx.FakeRunner) (core.Result, error) {
	t.Helper()
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return g.Execute(execx.WithRunner(context.Background(), fake), c, req)
}

func TestInterfaceFit(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if missing := g.Capabilities().Missing(); len(missing) != 0 {
		t.Fatalf("undeclared capabilities: %v", missing)
	}
	lang := golang.New()
	if g.Language() != lang.ID() || lang.DefaultPackageManager() != g.ID() {
		t.Fatalf("language/manager ids disagree: %q %q", g.Language(), lang.DefaultPackageManager())
	}
	if _, ok := core.Language(lang).(core.Templater); ok {
		t.Fatal("spike language must not implement Templater")
	}
	if got := g.Capabilities()[core.CapOutdated].Level; got != core.LevelUnsupported {
		t.Fatalf("outdated level = %s, want unsupported", got)
	}
}

func TestDetect(t *testing.T) {
	g, _ := New()
	lang := golang.New()
	mod := fstest.MapFS{"go.mod": {Data: []byte("module x\n")}}
	work := fstest.MapFS{"go.work": {Data: []byte("go 1.22\n")}}
	if !g.Detect(mod) || !lang.Detect(mod) || !lang.Detect(work) || g.Detect(work) {
		t.Fatal("go.mod must mark manager and language; go.work only the language")
	}
	if g.Detect(fstest.MapFS{"go.sum": {}}) {
		t.Fatal("go.sum is not a marker")
	}
}

func TestCommands(t *testing.T) {
	cases := []struct {
		name string
		cap  core.Capability
		req  core.Request
		want []string
	}{
		{"init", core.CapInit, core.Request{Name: "example.com/app"}, []string{"go mod init example.com/app"}},
		{"install", core.CapInstall, core.Request{}, []string{"go mod download"}},
		{"add", core.CapAdd, core.Request{Packages: []string{"github.com/a/b@v1.2.3"}}, []string{"go get github.com/a/b@v1.2.3"}},
		{"remove", core.CapRemove, core.Request{Packages: []string{"github.com/a/b"}}, []string{"go get github.com/a/b@none"}},
		{"run", core.CapRun, core.Request{Args: []string{"--flag"}}, []string{"go run . --flag"}},
		{"test", core.CapTest, core.Request{}, []string{"go test ./..."}},
		{"lint", core.CapLint, core.Request{}, []string{"go vet ./..."}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &execx.FakeRunner{}
			if _, err := run(t, c.cap, c.req, fake); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, call := range fake.Calls {
				got = append(got, call.String())
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("commands = %v, want %v", got, c.want)
			}
		})
	}
}

func TestInvalidRequests(t *testing.T) {
	for name, req := range map[string]core.Request{
		"group": {Packages: []string{"x"}, Group: "dev"},
		"empty": {},
	} {
		if _, err := run(t, core.CapAdd, req, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("add %s: err = %v, want ErrInvalidRequest", name, err)
		}
	}
	if _, err := run(t, core.CapRun, core.Request{Script: "dev"}, &execx.FakeRunner{}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("run with script: err = %v", err)
	}
	if _, err := run(t, core.CapPublish, core.Request{}, &execx.FakeRunner{}); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("publish: err = %v, want ErrUnsupported", err)
	}
}

func TestListParsesDirectDependencies(t *testing.T) {
	fake := &execx.FakeRunner{Responses: map[string]execx.Response{
		"go list -m -f " + listFormat + " all": {Output: execx.Output{Stdout: "\ngithub.com/a/b v1.2.3\ngithub.com/c/d v0.1.0\n"}},
	}}
	res, err := run(t, core.CapList, core.Request{}, fake)
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Name: "github.com/a/b", Version: "v1.2.3"}, {Name: "github.com/c/d", Version: "v0.1.0"}}
	if !reflect.DeepEqual(res.Data, want) {
		t.Fatalf("data = %v, want %v", res.Data, want)
	}
}

func TestDryRunListsCommandWithoutParsing(t *testing.T) {
	res, err := run(t, core.CapTest, core.Request{DryRun: true}, &execx.FakeRunner{})
	if err != nil || res.Summary == "" || len(res.Commands) != 1 {
		t.Fatalf("res = %+v err = %v", res, err)
	}
}
