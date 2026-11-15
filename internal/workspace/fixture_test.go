package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/justinfischman/universal-package-core/internal/components/runstate"
	"github.com/justinfischman/universal-package-core/internal/config"
)

// loadFixture loads testdata/two-apps: app api provides, app web
// consumes it.
func loadFixture(t *testing.T) *Workspace {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "two-apps"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, config.DirName, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	var c config.Config
	if err := toml.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	w, err := Load(root, c)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestFixtureOrderEnvFindingsAndRunState(t *testing.T) {
	w := loadFixture(t)
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
		if _, err := os.Stat(filepath.Join(a.Dir, "pyproject.toml")); err != nil {
			t.Errorf("app %s: %v", a.Name, err)
		}
	}
	api, _ := w.App("api")
	if a, b := (runstate.State{Dir: api.Dir, App: api.Name}).RunDir(), (runstate.State{Dir: web.Dir, App: web.Name}).RunDir(); a == b {
		t.Errorf("run directories clash: %s", a)
	}
}
