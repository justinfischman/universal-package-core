package envs_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/envs"
)

func TestDiscover(t *testing.T) {
	root := fstest.MapFS{
		".env.local":    {Data: []byte("A=1")},
		".env.example":  {Data: []byte("A=")},
		".env.bad name": {},
		".env":          {},
		"conf/prod.env": {},
		".env.dev":      {},
	}
	got, err := envs.Discover(root, map[string]string{"prod": "conf/prod.env", "dev": "", "gone": ".env.gone"})
	if err != nil {
		t.Fatal(err)
	}
	want := []envs.Entry{
		{Name: "dev", File: ".env.dev", Source: envs.SourceConfig, Exists: true},
		{Name: "gone", File: ".env.gone", Source: envs.SourceConfig},
		{Name: "local", File: ".env.local", Source: envs.SourceConvention, Exists: true},
		{Name: "prod", File: "conf/prod.env", Source: envs.SourceConfig, Exists: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	if err := envs.Check(got, "local"); err != nil {
		t.Error(err)
	}
	for _, name := range []string{"gone", "absent"} {
		if err := envs.Check(got, name); err == nil {
			t.Errorf("Check(%s) = nil", name)
		}
	}
}

func TestValidation(t *testing.T) {
	for _, n := range []string{"dev", "ci_1", "a-b"} {
		if envs.ValidName(n) != nil {
			t.Errorf("name %q rejected", n)
		}
	}
	for _, n := range []string{"", "../x", "a b", "-x", "a=b"} {
		if envs.ValidName(n) == nil {
			t.Errorf("name %q accepted", n)
		}
	}
	for _, f := range []string{".env.dev", "conf/dev.env"} {
		if envs.ValidFile(f) != nil {
			t.Errorf("file %q rejected", f)
		}
	}
	for _, f := range []string{"", "/etc/passwd", "../x", "a/../../x", `a\b`} {
		if envs.ValidFile(f) == nil {
			t.Errorf("file %q accepted", f)
		}
	}
}
