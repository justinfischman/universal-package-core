package python

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func mapFS(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return m
}

const pep723 = "# /// script\n# dependencies = []\n# ///\nprint('x')\n"

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"pyproject only", map[string]string{"pyproject.toml": "[project]\n"}, true},
		{"requirements only", map[string]string{"requirements.txt": "rich\n"}, true},
		{"setup.py only", map[string]string{"setup.py": "from setuptools import setup\n"}, true},
		{"pep 723 script", map[string]string{"tool.py": pep723}, true},
		{"plain python file", map[string]string{"tool.py": "print('x')\n"}, false},
		{"comment that is not a block", map[string]string{"tool.py": "# /// notscript\n"}, false},
		{"script block in a text file", map[string]string{"notes.txt": pep723}, false},
		{"empty directory", nil, false},
		{"unrelated files", map[string]string{"Cargo.toml": "[package]", "README.md": "hi"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := New().Detect(mapFS(c.files)); got != c.want {
				t.Errorf("Detect = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDetectIgnoresScriptsInSubdirectories(t *testing.T) {
	if New().Detect(mapFS(map[string]string{"sub/tool.py": pep723})) {
		t.Error("only top-level scripts mark a project")
	}
}

func TestScriptFilesSorted(t *testing.T) {
	got, err := ScriptFiles(mapFS(map[string]string{"b.py": pep723, "a.py": pep723, "c.py": "print()"}))
	if err != nil || !reflect.DeepEqual(got, []string{"a.py", "b.py"}) {
		t.Errorf("ScriptFiles = %v, %v", got, err)
	}
}

func TestIdentityAndManagers(t *testing.T) {
	l := New()
	if l.ID() != "python" || l.Kind() != core.KindLanguage {
		t.Errorf("identity = %q %q", l.ID(), l.Kind())
	}
	if !reflect.DeepEqual(l.PackageManagers(), []string{"uv", "pip", "poetry"}) || l.DefaultPackageManager() != "uv" {
		t.Errorf("managers = %v default %q", l.PackageManagers(), l.DefaultPackageManager())
	}
}

func TestTemplatesForScript(t *testing.T) {
	files, ok := New().Templates("script")
	if !ok {
		t.Fatal("no script templates")
	}
	data, err := fs.ReadFile(files, ".gitignore")
	if err != nil || !strings.Contains(string(data), "__pycache__") {
		t.Errorf(".gitignore = %q, %v", data, err)
	}
	if _, ok := New().Templates("spaceship"); ok {
		t.Error("unknown project type must have no templates")
	}
}

func TestTemplatesForPackage(t *testing.T) {
	files, ok := New().Templates("package")
	if !ok {
		t.Fatal("no package templates")
	}
	for _, name := range []string{".gitignore", "tests/test_package.py.tmpl"} {
		if _, err := fs.Stat(files, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestReadProject(t *testing.T) {
	fsys := mapFS(map[string]string{"pyproject.toml": `
[project]
name = "demo"
requires-python = ">=3.11"
[project.scripts]
serve = "demo:main"
both = "demo:a"
[tool.uv]
dev-dependencies = []
[tool.upc.scripts]
test = "pytest -q"
both = "echo upc"
`})
	p, err := ReadProject(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "demo" || p.RequiresPython != ">=3.11" || !p.HasToolUv {
		t.Errorf("project = %+v", p)
	}
	if p.ProjectScripts["serve"] != "demo:main" || p.UpcScripts["test"] != "pytest -q" {
		t.Errorf("scripts = %+v / %+v", p.ProjectScripts, p.UpcScripts)
	}

	m, err := Pyproject{}.Read(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"serve": "demo:main", "both": "echo upc", "test": "pytest -q"}
	if m.Name != "demo" || !reflect.DeepEqual(m.Scripts, want) {
		t.Errorf("manifest = %+v, want scripts %v (tool.upc wins)", m, want)
	}
	if (Pyproject{}).Filename() != "pyproject.toml" {
		t.Error("filename")
	}
}

func TestReadProjectErrors(t *testing.T) {
	if _, err := ReadProject(mapFS(nil)); err == nil {
		t.Error("missing file must error")
	}
	if _, err := ReadProject(mapFS(map[string]string{"pyproject.toml": "[[["})); err == nil {
		t.Error("broken TOML must error")
	}
	_, err := ReadProject(mapFS(map[string]string{"pyproject.toml": "[tool.upc.scripts]\nx = 3\n"}))
	if err == nil {
		t.Error("non-string upc script must error")
	}
}

func TestReadProjectWithoutToolTables(t *testing.T) {
	p, err := ReadProject(mapFS(map[string]string{"pyproject.toml": "[project]\nname = 'x'\n"}))
	if err != nil || p.HasToolUv || len(p.UpcScripts) != 0 {
		t.Errorf("project = %+v, %v", p, err)
	}
}

func TestRequirementNames(t *testing.T) {
	for in, want := range map[string]string{
		"rich": "rich", "Rich_Text>=1": "rich-text", "a.b[x]==1": "a-b", "pkg @ https://x": "pkg", "": "", "-r x": "", "# c": "",
	} {
		if got := RequirementName(in); got != want {
			t.Errorf("RequirementName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadProjectDependenciesAndToolTables(t *testing.T) {
	fsys := fstest.MapFS{"pyproject.toml": {Data: []byte(`[project]
name = "d"
dependencies = ["Rich>=13", "typing_extensions"]
[project.optional-dependencies]
docs = ["sphinx"]
[dependency-groups]
dev = ["pytest", {include-group = "docs"}]
[tool.ruff]
[tool.upc.scripts]
a = "b"
`)}}
	p, err := ReadProject(fsys)
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"rich", "typing-extensions", "sphinx", "pytest"} {
		if !p.Dependencies[dep] {
			t.Errorf("missing dependency %q in %v", dep, p.Dependencies)
		}
	}
	if !p.ToolTables["ruff"] || !p.ToolTables["upc"] || p.ToolTables["uv"] {
		t.Errorf("tool tables = %v", p.ToolTables)
	}
	if got := p.Scripts().Commands["a"]; got != "b" {
		t.Errorf("scripts = %+v", p.Scripts())
	}
}

func TestReadProjectPoetryDependencies(t *testing.T) {
	fsys := fstest.MapFS{"pyproject.toml": {Data: []byte(`[tool.poetry.dependencies]
python = "^3.11"
Requests = "^2"
typing_extensions = { version = "*", optional = true }
[tool.poetry.group.dev.dependencies]
pytest = "^8"
[tool.poetry.group.lint.dependencies]
ruff = "*"
[tool.poetry.dev-dependencies]
black = "*"
`)}}
	p, err := ReadProject(fsys)
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range []string{"requests", "typing-extensions", "pytest", "ruff", "black"} {
		if !p.Dependencies[dep] {
			t.Errorf("missing dependency %q in %v", dep, p.Dependencies)
		}
	}
	if p.Dependencies["python"] || !p.ToolTables["poetry"] {
		t.Errorf("dependencies = %v, tool tables = %v", p.Dependencies, p.ToolTables)
	}
}
