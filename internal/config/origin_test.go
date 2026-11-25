package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

// originFixture lays out a global directory, a workspace root and an app.
type originFixture struct {
	t      *testing.T
	global string
	root   string
	app    string
}

func newOriginFixture(t *testing.T) originFixture {
	t.Helper()
	base := t.TempDir()
	f := originFixture{
		t:      t,
		global: filepath.Join(base, "home", config.DirName),
		root:   filepath.Join(base, "ws"),
		app:    filepath.Join(base, "ws", "app"),
	}
	if err := os.MkdirAll(f.app, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

// write stores a TOML file in dir, creating dir.
func (f originFixture) write(dir, name, body string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// setLanguage writes `language` into the file backing a layer. The root config
// also declares the workspace so the store finds the root above the app.
func (f originFixture) setLanguage(layer config.Layer, value string) {
	f.t.Helper()
	body := "language = \"" + value + "\"\n"
	rootDir := filepath.Join(f.root, config.DirName)
	appDir := filepath.Join(f.app, config.DirName)
	switch layer {
	case config.LayerGlobal:
		f.write(f.global, config.FileName, body)
	case config.LayerRoot:
		f.write(rootDir, config.FileName, body+"[workspace]\nmembers = [\"app\"]\n")
	case config.LayerRootLocal:
		f.write(rootDir, config.LocalName, body)
	case config.LayerApp:
		f.write(appDir, config.FileName, body)
	case config.LayerAppLocal:
		f.write(appDir, config.LocalName, body)
	}
}

func (f originFixture) store() config.Store {
	regs, _ := coretest.Registries()
	return config.Store{FS: fsx.OS{}, GlobalDir: f.global, ProjectRoot: f.app, Registries: regs}
}

func originOf(t *testing.T, origins []config.Origin, key string) config.Origin {
	t.Helper()
	for _, o := range origins {
		if o.Key == key {
			return o
		}
	}
	t.Fatalf("no origin for key %q", key)
	return config.Origin{}
}

// TestOriginsEachLayerWins sets the key in every layer up to the one under test
// and expects that layer to be reported (ADR-0011 precedence).
func TestOriginsEachLayerWins(t *testing.T) {
	tests := []struct {
		layer  config.Layer
		value  string
		masked bool
	}{
		{config.LayerGlobal, "global-v", false},
		{config.LayerRoot, "root config-v", false},
		{config.LayerRootLocal, config.MaskedValue, true},
		{config.LayerApp, "app config-v", false},
		{config.LayerAppLocal, config.MaskedValue, true},
		{config.LayerFlag, "flag-v", false},
	}
	for _, tc := range tests {
		t.Run(string(tc.layer), func(t *testing.T) {
			f := newOriginFixture(t)
			for _, layer := range config.LayerOrder {
				f.setLanguage(layer, string(layer)+"-v")
				if layer == tc.layer {
					break
				}
			}
			var flags map[string]string
			if tc.layer == config.LayerFlag {
				flags = map[string]string{config.KeyLanguage: "flag-v"}
			}
			origins, err := f.store().Origins(flags)
			if err != nil {
				t.Fatal(err)
			}
			got := originOf(t, origins, config.KeyLanguage)
			want := config.Origin{Key: config.KeyLanguage, Value: tc.value, Layer: tc.layer, Masked: tc.masked}
			if got != want {
				t.Errorf("origin = %+v, want %+v", got, want)
			}
		})
	}
}

func TestOriginsUnsetKeyHasNoLayer(t *testing.T) {
	f := newOriginFixture(t)
	origins, err := f.store().Origins(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(origins) != len(config.Keys) {
		t.Fatalf("got %d origins, want %d", len(origins), len(config.Keys))
	}
	for _, o := range origins {
		if o.Layer != "" || o.Value != "" || o.Masked {
			t.Errorf("origin %+v, want unset", o)
		}
	}
}

func TestOriginsNeverExposesLocalSecret(t *testing.T) {
	f := newOriginFixture(t)
	f.write(filepath.Join(f.app, config.DirName), config.LocalName, "language = \"hunter2\"\n")
	origins, err := f.store().Origins(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range origins {
		if o.Value == "hunter2" {
			t.Fatalf("local value leaked: %+v", o)
		}
	}
	got := originOf(t, origins, config.KeyLanguage)
	if got.Layer != config.LayerAppLocal || !got.Masked || got.Value != config.MaskedValue {
		t.Errorf("origin = %+v, want masked app local", got)
	}
}

func TestOriginsFillGapsPerKey(t *testing.T) {
	f := newOriginFixture(t)
	f.write(f.global, config.FileName, "language = \"g\"\nproject_type = \"gp\"\n")
	f.write(filepath.Join(f.app, config.DirName), config.FileName, "language = \"a\"\n")
	origins, err := f.store().Origins(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := originOf(t, origins, config.KeyLanguage); got.Layer != config.LayerApp || got.Value != "a" {
		t.Errorf("language = %+v", got)
	}
	if got := originOf(t, origins, config.KeyProjectType); got.Layer != config.LayerGlobal || got.Value != "gp" {
		t.Errorf("project_type = %+v, want the walk to fill the gap from global", got)
	}
}
