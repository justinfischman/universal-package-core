package node

import (
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

func mapFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"manifest", map[string]string{"package.json": "{}"}, true},
		{"tsconfig only", map[string]string{"tsconfig.json": "{}"}, true},
		{"npm lock file", map[string]string{"package-lock.json": "{}"}, true},
		{"classic yarn lock file", map[string]string{"yarn.lock": ""}, true},
		{"plain source file", map[string]string{"index.ts": "export {}"}, false},
		{"other language", map[string]string{"pyproject.toml": "[project]\n"}, false},
		{"empty directory", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := New().Detect(mapFS(c.files)); got != c.want {
				t.Errorf("Detect = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsTypeScript(t *testing.T) {
	if !IsTypeScript(mapFS(map[string]string{"tsconfig.json": "{}"})) {
		t.Error("tsconfig.json marks TypeScript")
	}
	if IsTypeScript(mapFS(map[string]string{"package.json": "{}"})) {
		t.Error("a bare manifest is not TypeScript")
	}
}

func TestLanguageContract(t *testing.T) {
	regs := core.NewRegistries()
	for _, id := range []string{"npm", "yarn"} {
		pm, err := coretest.NewPackageManager(id, ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := regs.PackageManagers.Register(pm); err != nil {
			t.Fatal(err)
		}
	}
	contract.RunLanguageSuite(t, New(), regs)
}

func TestTemplatesPerProjectType(t *testing.T) {
	want := map[string][]string{
		"script":  {".gitignore", "tsconfig.json"},
		"package": {".gitignore", "src/index.ts", "test/index.test.ts.tmpl", "tsconfig.json"},
	}
	for projectType, files := range want {
		sub, ok := New().Templates(projectType)
		if !ok {
			t.Fatalf("no templates for %s", projectType)
		}
		var got []string
		err := fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				got = append(got, p)
			}
			return err
		})
		if err != nil || !reflect.DeepEqual(got, files) {
			t.Errorf("%s templates = %v (%v), want %v", projectType, got, err, files)
		}
	}
	if _, ok := New().Templates("app"); ok {
		t.Error("app has no templates")
	}
}
