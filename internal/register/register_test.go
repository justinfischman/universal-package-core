package register_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/contract"
	"github.com/justinfischman/universal-package-core/internal/register"
)

func defaults(t *testing.T) core.Registries {
	t.Helper()
	regs, err := register.Default()
	if err != nil {
		t.Fatal(err)
	}
	return regs
}

// The contract suites run on the real modules: capability parity, language and
// manager cross-references, project type identity.
func TestContractSuitesPassOnRealModules(t *testing.T) {
	contract.RunAll(t, defaults(t))
}

func TestRegisteredModules(t *testing.T) {
	regs := defaults(t)
	if got := regs.Languages.IDs(); !reflect.DeepEqual(got, []string{"node", "python"}) {
		t.Errorf("languages = %v", got)
	}
	if got := regs.PackageManagers.IDs(); !reflect.DeepEqual(got, []string{"npm", "pip", "poetry", "uv", "yarn"}) {
		t.Errorf("package managers = %v", got)
	}
	if got := regs.ProjectTypes.IDs(); !reflect.DeepEqual(got, []string{"app", "package", "script"}) {
		t.Errorf("project types = %v", got)
	}
	if regs.DefaultLanguage != "python" {
		t.Errorf("default language = %q", regs.DefaultLanguage)
	}
}

func TestEveryManagerDeclaresEveryCapability(t *testing.T) {
	for _, pm := range defaults(t).PackageManagers.List() {
		if missing := pm.Capabilities().Missing(); len(missing) != 0 {
			t.Errorf("%s misses %v", pm.ID(), missing)
		}
	}
}

func TestScriptTypeSupportsTheRegisteredStack(t *testing.T) {
	regs := defaults(t)
	st, _ := regs.ProjectTypes.Get("script")
	if !st.Supports("python", "uv") || !st.Supports("python", "") {
		t.Error("script must support python/uv")
	}
}

func TestAppTypeSupportsEveryScaffoldingPair(t *testing.T) {
	regs := defaults(t)
	pt, ok := regs.ProjectTypes.Get("app")
	if !ok {
		t.Fatal("app must be registered")
	}
	for _, pair := range [][2]string{
		{"python", "uv"}, {"python", "pip"}, {"python", "poetry"}, {"python", ""},
		{"node", "npm"}, {"node", "yarn"}, {"node", ""},
	} {
		if !pt.Supports(pair[0], pair[1]) {
			t.Errorf("app must support %s/%s", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{{"python", "npm"}, {"node", "uv"}, {"node", "pip"}, {"go", ""}, {"node", "pnpm"}} {
		if pt.Supports(pair[0], pair[1]) {
			t.Errorf("app must not be offered for %s/%s", pair[0], pair[1])
		}
	}
}

func TestProjectTypesSupportTheNodeStack(t *testing.T) {
	regs := defaults(t)
	for _, id := range []string{"script", "package", "app"} {
		pt, _ := regs.ProjectTypes.Get(id)
		if !pt.Supports("node", "npm") || !pt.Supports("node", "yarn") || !pt.Supports("node", "") {
			t.Errorf("%s must support node/npm and node/yarn", id)
		}
		if pt.Supports("node", "uv") || pt.Supports("python", "npm") {
			t.Errorf("%s must refuse a manager of another language", id)
		}
	}
}

func TestDefaultsAreIndependent(t *testing.T) {
	a, b := defaults(t), defaults(t)
	if a.Languages == b.Languages {
		t.Error("each call must build fresh registries")
	}
}

// Real managers rank overlapping markers: lockfile over tool config over a bare
// requirements.txt. The CLI keeps only the top rank and asks on a tie.
func TestPythonDetectionPriority(t *testing.T) {
	const pyproject = "[project]\nname = \"d\"\n"
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string // managers left after ranking, sorted by id
	}{
		{"requirements only", fstest.MapFS{"requirements.txt": {}}, []string{"pip"}},
		{"uv.lock beats requirements.txt", fstest.MapFS{"uv.lock": {}, "requirements.txt": {}}, []string{"uv"}},
		{"tool.uv beats requirements.txt", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n")}, "requirements.txt": {}}, []string{"uv"}},
		{"plain pyproject with requirements.txt is pip", fstest.MapFS{"pyproject.toml": {Data: []byte(pyproject)}, "requirements.txt": {}}, []string{"pip"}},
		{"poetry.lock beats requirements.txt", fstest.MapFS{"poetry.lock": {}, "requirements.txt": {}}, []string{"poetry"}},
		{"tool.poetry beats requirements.txt", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.poetry]\n")}, "requirements.txt": {}}, []string{"poetry"}},
		{"poetry.lock beats tool.uv", fstest.MapFS{"poetry.lock": {}, "pyproject.toml": {Data: []byte("[tool.uv]\n")}}, []string{"poetry"}},
		{"uv.lock beats tool.poetry", fstest.MapFS{"uv.lock": {}, "pyproject.toml": {Data: []byte("[tool.poetry]\n")}}, []string{"uv"}},
		{"both lockfiles tie", fstest.MapFS{"uv.lock": {}, "poetry.lock": {}}, []string{"poetry", "uv"}},
		{"both tool tables tie", fstest.MapFS{"pyproject.toml": {Data: []byte("[tool.uv]\n[tool.poetry]\n")}}, []string{"poetry", "uv"}},
		{"plain pyproject matches no manager", fstest.MapFS{"pyproject.toml": {Data: []byte(pyproject)}}, nil},
		{"nothing", fstest.MapFS{}, nil},
	}
	regs := defaults(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			best, got := -1, []string(nil)
			for _, pm := range regs.PackageManagers.List() {
				if !pm.Detect(c.files) {
					continue
				}
				rank := 0
				if r, ok := pm.(core.DetectRanker); ok {
					rank = r.DetectRank(c.files)
				}
				switch {
				case rank > best:
					best, got = rank, []string{pm.ID()}
				case rank == best:
					got = append(got, pm.ID())
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("managers = %v, want %v", got, c.want)
			}
		})
	}
}
