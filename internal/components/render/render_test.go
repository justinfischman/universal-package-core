package render_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/fsx"
	"github.com/justinfischman/universal-package-core/internal/components/render"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var tmpl = fstest.MapFS{
	"README.md.tmpl":   {Data: []byte("# {{.Name}}\n")},
	"src/static.txt":   {Data: []byte("{{not a template}}")},
	"src/main.py.tmpl": {Data: []byte("print({{printf \"%q\" .Name}})\n")},
}

func TestRenderWritesFiles(t *testing.T) {
	dest := t.TempDir()
	written, err := render.Render(fsx.OS{}, tmpl, dest, map[string]string{"Name": "demo"}, render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 3 {
		t.Errorf("written = %v", written)
	}
	check := map[string]string{
		"README.md":      "# demo\n",
		"src/main.py":    "print(\"demo\")\n",
		"src/static.txt": "{{not a template}}",
	}
	for name, want := range check {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v", name, got, err)
		}
	}
}

func TestRenderRefusesOverwrite(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "README.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := render.Render(fsx.OS{}, tmpl, dest, map[string]string{"Name": "x"}, render.Options{})
	if !errors.Is(err, core.ErrInvalidRequest) {
		t.Fatalf("got %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "README.md")); string(got) != "mine" {
		t.Error("existing file modified")
	}
	if _, err := os.Stat(filepath.Join(dest, "src")); err == nil {
		t.Error("partial output written before conflict check")
	}
	if _, err := render.Render(fsx.OS{}, tmpl, dest, map[string]string{"Name": "x"}, render.Options{Overwrite: true}); err != nil {
		t.Errorf("overwrite allowed: %v", err)
	}
}

func TestRenderMissingKeyLeavesNothing(t *testing.T) {
	dest := t.TempDir()
	_, err := render.Render(fsx.OS{}, tmpl, dest, map[string]string{}, render.Options{})
	if err == nil || !strings.Contains(err.Error(), "render template") {
		t.Fatalf("got %v", err)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Errorf("leftovers: %v", entries)
	}
}

// failingFS fails the second write to prove rollback.
type failingFS struct {
	fsx.OS
	writes int
}

func (f *failingFS) WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	f.writes++
	if f.writes == 2 {
		return errors.New("disk full")
	}
	return f.OS.WriteFileAtomic(path, data, perm)
}

func TestRenderRollsBackOnWriteFailure(t *testing.T) {
	dest := t.TempDir()
	_, err := render.Render(&failingFS{}, tmpl, dest, map[string]string{"Name": "x"}, render.Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	var leftovers []string
	_ = filepath.WalkDir(dest, func(p string, d fs.DirEntry, _ error) error {
		if !d.IsDir() {
			leftovers = append(leftovers, p)
		}
		return nil
	})
	if len(leftovers) != 0 {
		t.Errorf("not rolled back: %v", leftovers)
	}
}

func TestRenderEmptySource(t *testing.T) {
	if _, err := render.Render(fsx.OS{}, fstest.MapFS{}, t.TempDir(), nil, render.Options{}); err == nil {
		t.Error("expected error for empty template")
	}
}
