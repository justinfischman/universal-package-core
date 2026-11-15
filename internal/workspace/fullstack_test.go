package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
)

// testdata/fullstack is two apps on different managers; the manager identity
// lives only in their manifests, never in Go code (internal/arch).
func TestFullstackFixtureOrderAndEnv(t *testing.T) {
	w, _ := loadFixtureNamed(t, "fullstack")
	order, err := w.Order()
	if err != nil || !reflect.DeepEqual(names(order), []string{"api", "web"}) {
		t.Fatalf("order = %v, %v", names(order), err)
	}
	web, _ := w.App("web")
	if env, err := w.ChildEnv(web); err != nil || !reflect.DeepEqual(env, []string{"API_URL=http://localhost:8000/v1"}) {
		t.Fatalf("env = %v, %v", env, err)
	}
	if f := w.Findings(); len(f) != 0 {
		t.Fatalf("findings = %v", f)
	}
	for _, a := range order {
		entries, err := os.ReadDir(a.Dir)
		if err != nil || len(entries) != 1 {
			t.Errorf("app %s: want exactly one manifest in %s, got %v (%v)", a.Name, filepath.Base(a.Dir), entries, err)
		}
	}
}

func TestFullstackFixtureDoctorFindings(t *testing.T) {
	cases := map[string]struct {
		alter func(c *config.Config)
		want  FindingKind
	}{
		"port collision": {
			alter: func(c *config.Config) { a := c.Apps["web"]; a.Port = 8000; c.Apps["web"] = a },
			want:  PortCollision,
		},
		"consumer without provider": {
			alter: func(c *config.Config) { a := c.Apps["api"]; a.Provides = nil; c.Apps["api"] = a },
			want:  ConsumerWithoutProvider,
		},
		"unresolved variable": {
			alter: func(c *config.Config) {
				a := c.Apps["web"]
				a.Env = map[string]string{"API_URL": "${apps.ghost.url}"}
				c.Apps["web"] = a
			},
			want: UnresolvedVariable,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			base, c := loadFixtureNamed(t, "fullstack")
			tc.alter(&c)
			w, err := Load(base.Root, c)
			if err != nil {
				t.Fatal(err)
			}
			var kinds []FindingKind
			for _, f := range w.Findings() {
				kinds = append(kinds, f.Kind)
			}
			if !reflect.DeepEqual(kinds, []FindingKind{tc.want}) {
				t.Fatalf("finding kinds = %v, want [%s]", kinds, tc.want)
			}
		})
	}
}
