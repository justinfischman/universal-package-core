package arch_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/arch"
	"github.com/justinfischman/universal-package-core/internal/register"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("locate module root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func listPackages(t *testing.T, root string) []arch.Pkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v: %s", err, stderr.String())
	}
	var pkgs []arch.Pkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p arch.Pkg
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func TestImportRulesHoldForModule(t *testing.T) {
	pkgs := listPackages(t, moduleRoot(t))
	if len(pkgs) < 5 {
		t.Fatalf("suspiciously few packages listed: %d", len(pkgs))
	}
	for _, v := range arch.CheckImports(pkgs) {
		t.Error(v)
	}
}

// Language and package manager ids must not appear outside their own tree.
// Project type ids ("script", "package", "app") are ordinary words in Go and in
// core's vocabulary, so they are covered by the import rules instead.
func TestNoModuleNamesLeakOutsideTheirTree(t *testing.T) {
	regs, err := register.Default()
	if err != nil {
		t.Fatal(err)
	}
	ids := append(regs.Languages.IDs(), regs.PackageManagers.IDs()...)
	violations, err := arch.CheckWords(moduleRoot(t), ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Error(v)
	}
}

// Marker file names (lock files, manifests) belong to their language's tree.
func TestNoMarkerFilesLeakOutsideLanguageTree(t *testing.T) {
	regs, err := register.Default()
	if err != nil {
		t.Fatal(err)
	}
	violations, err := arch.CheckMarkers(moduleRoot(t), regs.MarkerFiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Error(v)
	}
}

func TestCheckMarkersFindsLeaks(t *testing.T) {
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
	write("internal/cli/x.go", `package cli // reads thing.lock`)
	write("internal/stacks/lang/x.go", `package lang // thing.lock is fine here`)
	got, err := arch.CheckMarkers(root, map[string]string{"thing.lock": "lang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "internal/cli/x.go") {
		t.Errorf("got %v", got)
	}
}

func TestCheckImportsDetectsViolations(t *testing.T) {
	m := arch.ModulePath
	cases := []struct {
		name string
		pkg  arch.Pkg
		want string
	}{
		{"core imports a language", arch.Pkg{ImportPath: m + "internal/core", Imports: []string{m + "internal/stacks/python"}}, "core may import only"},
		{"core imports config", arch.Pkg{ImportPath: m + "internal/core/contract", Imports: []string{m + "internal/config"}}, "core may import only"},
		{"language imports other language", arch.Pkg{ImportPath: m + "internal/stacks/python/uv", Imports: []string{m + "internal/stacks/node/npm"}}, "must not import each other"},
		{"project type imports a language", arch.Pkg{ImportPath: m + "internal/projects/script", Imports: []string{m + "internal/stacks/python"}}, "must not import each other"},
		{"language imports register", arch.Pkg{ImportPath: m + "internal/stacks/python", Imports: []string{m + "internal/register"}}, "wiring or the CLI"},
		{"cli imports a language", arch.Pkg{ImportPath: m + "internal/cli", Imports: []string{m + "internal/stacks/python"}}, "only internal/register may"},
		{"cli imports register", arch.Pkg{ImportPath: m + "internal/cli", Imports: []string{m + "internal/register"}}, "only cmd/ may"},
		{"test imports foreign module", arch.Pkg{ImportPath: m + "internal/config", XTestImports: []string{m + "internal/projects/app"}}, "only internal/register may"},
	}
	for _, c := range cases {
		got := arch.CheckImports([]arch.Pkg{c.pkg})
		if len(got) != 1 || !strings.Contains(got[0], c.want) {
			t.Errorf("%s: got %v, want one violation containing %q", c.name, got, c.want)
		}
	}
}

func TestCheckImportsAllowsLegalEdges(t *testing.T) {
	m := arch.ModulePath
	legal := []arch.Pkg{
		{ImportPath: m + "cmd/upc", Imports: []string{m + "internal/register", m + "internal/cli", "fmt"}},
		{ImportPath: m + "internal/register", Imports: []string{m + "internal/stacks/python", m + "internal/projects/script", m + "internal/core"}},
		{ImportPath: m + "internal/stacks/python/uv", Imports: []string{m + "internal/stacks/python", m + "internal/core", m + "internal/components/execx"}},
		{ImportPath: m + "internal/core", Imports: []string{m + "internal/components/execx"}},
		{ImportPath: m + "internal/cli", Imports: []string{m + "internal/core", m + "internal/config"}},
	}
	if got := arch.CheckImports(legal); len(got) != 0 {
		t.Errorf("legal edges flagged: %v", got)
	}
}

func TestCheckWordsFindsLeaksAndAllowsOwnTree(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		writeFile(t, filepath.Join(root, rel), body)
	}
	write("internal/stacks/python/uv/uv.go", "package uv // Python and uv live here\n")
	write("internal/cli/leak.go", "package cli // calls Python directly\n")
	write("internal/cli/leak2.go", "package cli // runs uv add\n")
	write("internal/cli/fine.go", "package cli // pythonic is a different word\n")
	write("internal/register/register.go", "package register // python, uv\n")
	write("docs/notes.go", "package docs // python\n")

	got, err := arch.CheckWords(root, []string{"python", "uv"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"internal/cli/leak.go mentions \"python\"", "internal/cli/leak2.go mentions \"uv\""} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing violation %q in:\n%s", want, joined)
		}
	}
	if len(got) != 2 {
		t.Errorf("want exactly 2 violations, got %d:\n%s", len(got), joined)
	}
}

func TestCheckWordsAllowsPackageManagerInOwningLanguageTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "internal/stacks/python/uv/uv.go"), "package uv\n")
	writeFile(t, filepath.Join(root, "internal/stacks/python/python.go"), "package python // declares uv\n")
	writeFile(t, filepath.Join(root, "internal/stacks/node/node.go"), "package node // mentions uv\n")
	got, err := arch.CheckWords(root, []string{"uv"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "internal/stacks/node/node.go mentions \"uv\"") {
		t.Errorf("want only the node leak, got %v", got)
	}
}
