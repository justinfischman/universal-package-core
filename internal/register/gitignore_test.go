package register_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// ADR-0014 decision 6: the contract locks are committed; local.toml and run/
// are not. Every language's generated .gitignore must say so, checked with the
// real `git check-ignore`.
func TestGeneratedGitignoreKeepsLocksAndIgnoresLocalAndRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	cases := []struct {
		path    string
		ignored bool
	}{
		{".universal-package-core/contracts/api.lock", false},
		{".universal-package-core/config.toml", false},
		{".universal-package-core/local.toml", true},
		{".universal-package-core/run/up.pid", true},
		{"apps/api/.universal-package-core/run/api/up.log", true},
	}
	for _, lang := range defaults(t).Languages.List() {
		tpl, ok := lang.(core.Templater)
		if !ok {
			continue
		}
		for _, projectType := range []string{"script", "package"} {
			files, ok := tpl.Templates(projectType)
			if !ok {
				continue
			}
			t.Run(lang.ID()+"/"+projectType, func(t *testing.T) {
				data, err := fs.ReadFile(files, ".gitignore")
				if err != nil {
					t.Fatal(err)
				}
				repo := t.TempDir()
				if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
					t.Fatalf("git init: %v: %s", err, out)
				}
				if err := os.WriteFile(filepath.Join(repo, ".gitignore"), data, 0o644); err != nil {
					t.Fatal(err)
				}
				for _, c := range cases {
					err := exec.Command("git", "-C", repo, "check-ignore", "-q", c.path).Run()
					if got := err == nil; got != c.ignored {
						t.Errorf("check-ignore %s = %v, want %v (.gitignore:\n%s)", c.path, got, c.ignored, data)
					}
				}
			})
		}
	}
}
