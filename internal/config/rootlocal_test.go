package config_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/config"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func mergedLanguage(t *testing.T, f originFixture) string {
	t.Helper()
	res, err := f.store().LoadMerged()
	if err != nil {
		t.Fatal(err)
	}
	return res.Language
}

// TestLoadMergedRootLocalPrecedence checks the root local.toml sits above the
// root config and below the app config, matching LayerOrder (ADR-0011).
func TestLoadMergedRootLocalPrecedence(t *testing.T) {
	f := newOriginFixture(t)
	f.setLanguage(config.LayerRoot, "lang-a")
	if got := mergedLanguage(t, f); got != "lang-a" {
		t.Fatalf("root config only: language = %q, want lang-a", got)
	}
	f.setLanguage(config.LayerRootLocal, "lang-b")
	if got := mergedLanguage(t, f); got != "lang-b" {
		t.Errorf("root local over root: language = %q, want lang-b", got)
	}
	f.setLanguage(config.LayerApp, "lang-a")
	if got := mergedLanguage(t, f); got != "lang-a" {
		t.Errorf("app over root local: language = %q, want lang-a", got)
	}
}

// TestLoadMergedRootLocalMergesEnvironments checks tables from the root local
// layer merge by key with the root config's.
func TestLoadMergedRootLocalMergesEnvironments(t *testing.T) {
	f := newOriginFixture(t)
	rootDir := filepath.Join(f.root, config.DirName)
	f.write(rootDir, config.FileName, "[workspace]\nmembers = [\"app\"]\n[environments.dev]\nfile = \".env.dev\"\n")
	f.write(rootDir, config.LocalName, "[environments.secret]\nfile = \".env.secret\"\ngroups = [\"docs\"]\n")
	res, err := f.store().LoadMerged()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Environments) != 2 || res.Environments["secret"].Groups[0] != "docs" {
		t.Errorf("environments = %+v, want dev and secret with group docs", res.Environments)
	}
}

// TestLoadMergedRootLocalMalformed returns the parse error instead of ignoring
// the file, as for every other layer.
func TestLoadMergedRootLocalMalformed(t *testing.T) {
	f := newOriginFixture(t)
	f.setLanguage(config.LayerRoot, "lang-b")
	f.write(filepath.Join(f.root, config.DirName), config.LocalName, "language = [")
	_, err := f.store().LoadMerged()
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
}

// TestLoadMergedRootLocalIgnoredWithoutRoot checks that without a separate
// workspace root nothing is read from a root-level directory.
func TestLoadMergedRootLocalIgnoredWithoutRoot(t *testing.T) {
	f := newOriginFixture(t)
	f.setLanguage(config.LayerAppLocal, "lang-b")
	if got := mergedLanguage(t, f); got != "lang-b" {
		t.Errorf("app local only: language = %q, want lang-b", got)
	}
}
