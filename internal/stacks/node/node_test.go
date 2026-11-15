package node

import (
	"io/fs"
	"reflect"
	"strings"
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
		"app":     {".gitignore", "APP.md.tmpl", "Dockerfile", "compose.yaml", "src/main.ts", "tsconfig.json"},
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
}

func TestAppTemplatesGolden(t *testing.T) {
	files, ok := New().Templates("app")
	if !ok {
		t.Fatal("node must provide app templates")
	}
	read := func(name string) string {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	if got, want := read(".gitignore"), "node_modules/\ndist/\n.universal-package-core/run/\n"; got != want {
		t.Errorf(".gitignore = %q, want %q", got, want)
	}
	wants := map[string][]string{
		"src/main.ts":   {`from "node:http"`, `process.argv[2] ?? "8000"`, `"127.0.0.1"`, "APP_HOST"},
		"tsconfig.json": {`"strict": true`, `"module": "nodenext"`, `"outDir": "dist"`, `"rootDir": "src"`, `"erasableSyntaxOnly": true`},
		"Dockerfile":    {"npm run build", "ENV APP_HOST=0.0.0.0", "EXPOSE 8000", `CMD ["node", "dist/main.js"`},
		"compose.yaml":  {"services:", "build: .", `"127.0.0.1:8000:8000"`},
		"APP.md.tmpl":   {"# {{.Name}}", "upc run dev", "upc build"},
	}
	for name, needs := range wants {
		body := read(name)
		for _, need := range needs {
			if !strings.Contains(body, need) {
				t.Errorf("%s lacks %s", name, need)
			}
		}
	}
	if strings.Contains(read("tsconfig.json"), `"declaration": true`) {
		t.Error("an app emits no declaration files")
	}
}
