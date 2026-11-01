package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

func newStore(t *testing.T) config.Store {
	t.Helper()
	root := t.TempDir()
	return config.Store{FS: fsx.OS{}, GlobalDir: filepath.Join(root, "home", config.DirName), ProjectRoot: filepath.Join(root, "proj")}
}

func TestGlobalDir(t *testing.T) {
	home := func() (string, error) { return "/home/u", nil }
	got, _ := config.GlobalDir(func(string) string { return "" }, home)
	if got != filepath.Join("/home/u", config.DirName) {
		t.Errorf("default = %q", got)
	}
	got, _ = config.GlobalDir(func(k string) string {
		if k == config.HomeEnv {
			return "/custom"
		}
		return ""
	}, home)
	if got != "/custom" {
		t.Errorf("override = %q", got)
	}
	if _, err := config.GlobalDir(func(string) string { return "" }, func() (string, error) { return "", errors.New("no home") }); err == nil {
		t.Error("expected home error")
	}
}

func TestMissingFilesYieldEmptyConfig(t *testing.T) {
	res, err := newStore(t).Load()
	if err != nil || res.Language != "" || len(res.Sources) != 0 {
		t.Errorf("got %+v, %v", res, err)
	}
}

func TestProjectOverridesGlobalFieldByField(t *testing.T) {
	s := newStore(t)
	if err := s.Save(config.Global, config.Config{Language: "alpha", PackageManager: "alpha-one", Environments: map[string]string{"dev": ".env.dev", "prod": ".env.prod"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(config.Project, config.Config{PackageManager: "alpha-two", Environments: map[string]string{"dev": ".env.local"}}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if res.Language != "alpha" || res.Sources[config.KeyLanguage] != config.Global {
		t.Errorf("language: %q from %q", res.Language, res.Sources[config.KeyLanguage])
	}
	if res.PackageManager != "alpha-two" || res.Sources[config.KeyPackageManager] != config.Project {
		t.Errorf("pm: %q from %q", res.PackageManager, res.Sources[config.KeyPackageManager])
	}
	if res.Environments["dev"] != ".env.local" || res.Environments["prod"] != ".env.prod" {
		t.Errorf("environments = %v", res.Environments)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	s := newStore(t)
	path := filepath.Join(s.ProjectRoot, config.DirName, config.FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("languag = \"typo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("got %v", err)
	}
}

func TestSaveCreatesDirLazily(t *testing.T) {
	s := newStore(t)
	if _, err := os.Stat(s.GlobalDir); err == nil {
		t.Fatal("dir exists before save")
	}
	if err := s.Save(config.Global, config.Config{Language: "alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.GlobalDir, config.FileName)); err != nil {
		t.Error(err)
	}
	if err := (config.Store{FS: fsx.OS{}, GlobalDir: s.GlobalDir}).Save(config.Project, config.Config{}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("project save without root: %v", err)
	}
}

func TestGetSet(t *testing.T) {
	c, err := config.Config{}.Set(config.KeyPackageManager, "uv")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get(config.KeyPackageManager); v != "uv" {
		t.Errorf("got %q", v)
	}
	if _, err := c.Set("bogus", "x"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("set bogus: %v", err)
	}
	if _, err := c.Get("bogus"); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("get bogus: %v", err)
	}
}

func TestValidateMatrix(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		cfg  config.Config
		want error
	}{
		{"empty", config.Config{}, nil},
		{"valid", config.Config{Language: "alpha", PackageManager: "alpha-two", ProjectType: "gadget"}, nil},
		{"pm only", config.Config{PackageManager: "beta-one"}, nil},
		{"unknown language", config.Config{Language: "nope"}, core.ErrInvalidRequest},
		{"unknown pm", config.Config{PackageManager: "nope"}, core.ErrInvalidRequest},
		{"unknown type", config.Config{ProjectType: "nope"}, core.ErrInvalidRequest},
		{"cross-language pm", config.Config{Language: "alpha", PackageManager: "beta-one"}, core.ErrIncompatible},
	}
	for _, c := range cases {
		err := config.Validate(c.cfg, regs)
		if !errors.Is(err, c.want) || (c.want == nil && err != nil) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestFindRoot(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	global := filepath.Join(base, config.DirName)
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{base}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}

	byConfig := mk("a", "b", "c")
	mk("a", config.DirName)
	if got := config.FindRoot(fsx.OS{}, byConfig, global, regs); got != filepath.Join(base, "a") {
		t.Errorf("config dir root = %q", got)
	}

	byMarker := mk("x", "y")
	if err := os.WriteFile(filepath.Join(base, "x", "alpha-one.marker"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := config.FindRoot(fsx.OS{}, byMarker, global, regs); got != filepath.Join(base, "x") {
		t.Errorf("marker root = %q", got)
	}

	none := mk("lonely")
	if got := config.FindRoot(fsx.OS{}, none, global, regs); got != "" {
		t.Errorf("global config mistaken for project: %q", got)
	}
}
