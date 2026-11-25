package register_test

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

var update = flag.Bool("update", false, "rewrite golden files")

// rank mirrors the CLI's auto-detection: keep the managers of the highest rank.
func rank(t *testing.T, files fstest.MapFS) []string {
	t.Helper()
	best, got := -1, []string(nil)
	for _, pm := range defaults(t).PackageManagers.List() {
		if !pm.Detect(files) {
			continue
		}
		r := 0
		if ranker, ok := pm.(core.DetectRanker); ok {
			r = ranker.DetectRank(files)
		}
		switch {
		case r > best:
			best, got = r, []string{pm.ID()}
		case r == best:
			got = append(got, pm.ID())
		}
	}
	return got
}

func TestNodeDetectionPriority(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"manifest only", fstest.MapFS{"package.json": {}}, []string{"npm"}},
		{"lock file", fstest.MapFS{"package-lock.json": {}}, []string{"npm"}},
		{"manifest with lock file", fstest.MapFS{"package.json": {}, "package-lock.json": {}}, []string{"npm"}},
		{"tsconfig alone matches no manager", fstest.MapFS{"tsconfig.json": {}}, nil},
		{"yarn lock alone", fstest.MapFS{"yarn.lock": {}}, []string{"yarn"}},
		{"yarn lock beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "yarn.lock": {}}, []string{"yarn"}},
		{"yarn lock ties package-lock.json", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "yarn.lock": {}, "package-lock.json": {}}, []string{"npm", "yarn"}},
		{"packageManager yarn beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"yarn@1.22.22"}`)}}, []string{"yarn"}},
		{"package-lock.json beats packageManager yarn", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"yarn@1.22.22"}`)}, "package-lock.json": {}}, []string{"npm"}},
		{"pnpm lock alone", fstest.MapFS{"pnpm-lock.yaml": {}}, []string{"pnpm"}},
		{"pnpm lock beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "pnpm-lock.yaml": {}}, []string{"pnpm"}},
		{"packageManager pnpm beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"pnpm@10.0.0"}`)}}, []string{"pnpm"}},
		{"bun lock alone", fstest.MapFS{"bun.lock": {}}, []string{"bun"}},
		{"legacy bun lockb beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "bun.lockb": {}}, []string{"bun"}},
		{"packageManager bun beats manifest", fstest.MapFS{"package.json": {Data: []byte(`{"packageManager":"bun@1.2.0"}`)}}, []string{"bun"}},
		{"pnpm lock ties yarn lock", fstest.MapFS{"package.json": {Data: []byte(`{}`)}, "pnpm-lock.yaml": {}, "yarn.lock": {}}, []string{"pnpm", "yarn"}},
		{"python project is not node", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n")}}, []string{"uv"}},
		{"two languages in one directory tie on languages, not on rank", fstest.MapFS{"uv.lock": {}, "package-lock.json": {}}, []string{"npm", "uv"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rank(t, c.files); !reflect.DeepEqual(got, c.want) {
				t.Errorf("managers = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNodeLanguageDetection(t *testing.T) {
	node, _ := defaults(t).Languages.Get("node")
	if !node.Detect(fstest.MapFS{"tsconfig.json": {}}) || node.Detect(fstest.MapFS{"pyproject.toml": {}}) {
		t.Error("node detects tsconfig.json and not a Python project")
	}
}

// scaffoldTree scaffolds a project with the real project type and language
// templates, with the manager's commands faked, and renders the result.
func scaffoldTree(t *testing.T, projectType, name string) string {
	t.Helper()
	regs := defaults(t)
	pt, _ := regs.ProjectTypes.Get(projectType)
	lang, _ := regs.Languages.Get("node")
	pm, _ := regs.PackageManagers.Get("npm")
	req := core.ScaffoldRequest{
		Request: core.Request{Dir: t.TempDir()},
		Name:    name, Language: "node", PackageManager: "npm", Manager: pm,
	}
	if files, ok := lang.(core.Templater).Templates(projectType); ok {
		req.Templates = files
	}
	fake := &execx.FakeRunner{}
	if _, err := pt.Scaffold(execx.WithRunner(context.Background(), fake), req); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range fake.Calls {
		fmt.Fprintf(&b, "$ %s\n", c)
	}
	root := filepath.Join(req.Dir, name)
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if p == "LICENSE" { // carries the current year
			fmt.Fprintf(&b, "\n=== %s (not compared)\n", p)
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "\n=== %s\n%s", p, data)
	}
	return strings.ReplaceAll(b.String(), req.Dir, "<dir>")
}

func TestNodeScaffoldGoldens(t *testing.T) {
	cases := []struct{ projectType, name, golden string }{
		{"script", "hello", "node_script.golden"},
		{"package", "demo-lib", "node_package.golden"},
	}
	for _, c := range cases {
		t.Run(c.projectType, func(t *testing.T) {
			got := scaffoldTree(t, c.projectType, c.name)
			path := filepath.Join("testdata", c.golden)
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != got {
				t.Errorf("%s mismatch\nwant:\n%s\ngot:\n%s", c.golden, want, got)
			}
		})
	}
}
