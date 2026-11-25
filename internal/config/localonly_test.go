package config_test

import (
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
)

// TestLoadSeesAppLocalWithoutConfigToml guards the bug where Load ignored
// local.toml, so `config get` reported (unset) while origin reporting found it.
func TestLoadSeesAppLocalWithoutConfigToml(t *testing.T) {
	f := newOriginFixture(t)
	f.setLanguage(config.LayerAppLocal, "custom-lang")
	s := f.store()

	res, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := res.Get(config.KeyLanguage); got != "custom-lang" {
		t.Errorf("Load language = %q, want custom-lang", got)
	}
	if res.Sources[config.KeyLanguage] != config.Project {
		t.Errorf("source = %q, want project", res.Sources[config.KeyLanguage])
	}

	origins, err := s.Origins(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o := originOf(t, origins, config.KeyLanguage); o.Layer != config.LayerAppLocal {
		t.Errorf("origin layer = %q, want app local", o.Layer)
	}
}

// TestLoadAgreesWithOriginsOnEveryLayer checks Load and Origins resolve the
// same effective value for each layer holding the key on its own.
func TestLoadAgreesWithOriginsOnEveryLayer(t *testing.T) {
	for _, layer := range []config.Layer{config.LayerGlobal, config.LayerRoot, config.LayerApp, config.LayerAppLocal} {
		t.Run(string(layer), func(t *testing.T) {
			f := newOriginFixture(t)
			f.setLanguage(layer, "custom-lang")
			s := f.store()
			res, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			origins, err := s.Origins(nil)
			if err != nil {
				t.Fatal(err)
			}
			o := originOf(t, origins, config.KeyLanguage)
			if o.Layer != layer {
				t.Fatalf("origin layer = %q, want %q", o.Layer, layer)
			}
			if got, _ := res.Get(config.KeyLanguage); got != "custom-lang" {
				t.Errorf("Load language = %q, want custom-lang", got)
			}
		})
	}
}
