package arch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/arch"
)

func TestNoToolLiteralsLeakOutsideStackPackages(t *testing.T) {
	got, err := arch.CheckToolLiterals(moduleRoot(t), arch.ToolOwners)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range got {
		t.Error(v)
	}
}

func TestCheckToolLiteralsFindsLeaks(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/core/x.go", "package core\nvar a = \"just\"\n")
	write("internal/cli/y.go", "package cli\n// just a comment about docker\nvar b = \"docker\"\n")
	write("internal/cli/y_test.go", "package cli\nvar c = \"compose\"\n")
	write("internal/stacks/just/j.go", "package just\nvar d = \"just\"\n")
	write("internal/stacks/compose/c.go", "package compose\nvar e = \"docker\"\nvar f = `compose`\n")
	got, err := arch.CheckToolLiterals(root, arch.ToolOwners)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "internal/core/x.go") || !strings.Contains(joined, "internal/cli/y.go") {
		t.Errorf("got %v", got)
	}
}
