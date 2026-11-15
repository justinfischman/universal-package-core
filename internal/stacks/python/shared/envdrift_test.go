package shared

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
)

func venvWith(dists ...string) fstest.MapFS {
	fsys := fstest.MapFS{".venv/pyvenv.cfg": {}}
	for _, d := range dists {
		fsys[".venv/lib/python3.12/site-packages/"+d+".dist-info/METADATA"] = &fstest.MapFile{}
	}
	return fsys
}

func TestInstalled(t *testing.T) {
	got, ok := Installed(venvWith("Rich-13.7.0", "typing_extensions-4.9.0"))
	want := map[string]string{"rich": "13.7.0", "typing-extensions": "4.9.0"}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("Installed = %v, %v", got, ok)
	}
	win := fstest.MapFS{".venv/Lib/site-packages/rich-1.0.dist-info/METADATA": {}}
	if got, ok := Installed(win); !ok || got["rich"] != "1.0" {
		t.Errorf("windows layout = %v, %v", got, ok)
	}
	if _, ok := Installed(fstest.MapFS{".venv/pyvenv.cfg": {}}); ok {
		t.Error("a venv without site-packages is unreadable, not empty")
	}
}

func TestRequirementPins(t *testing.T) {
	fsys := fstest.MapFS{
		"requirements.txt": {Data: []byte("# c\nrich==13.7.0  # pinned\nRequests>=2\n-r dev/base.txt\n-e .\n" +
			"colorama==0.4 ; sys_platform=='win32'\nlocal @ file:///x\nloose==1.*\n")},
		"dev/base.txt": {Data: []byte("pytest==8.0\n-r ../requirements.txt\n")},
	}
	want := map[string]string{"rich": "13.7.0", "requests": "", "pytest": "8.0", "loose": ""}
	if got := RequirementPins(fsys, "requirements.txt"); !reflect.DeepEqual(got, want) {
		t.Errorf("pins = %v, want %v", got, want)
	}
}

func TestEnvDrift(t *testing.T) {
	want := map[string]string{"rich": "13.7.0", "requests": "", "pytest": "8.0"}
	got := EnvDrift(venvWith("rich-12.0", "requests-2.31"), "requirements.txt", want)
	if len(got) != 2 {
		t.Fatalf("findings = %+v", got)
	}
	if !strings.Contains(got[0].Message, "pytest not installed") || !strings.Contains(got[1].Message, "rich (12.0 installed, 13.7.0 in requirements.txt)") {
		t.Errorf("messages = %q / %q", got[0].Message, got[1].Message)
	}
	if got[0].Fix != core.CapInstall || got[0].Severity != core.SeverityWarn {
		t.Errorf("finding = %+v", got[0])
	}
	if got := EnvDrift(venvWith("rich-13.7.0", "requests-2.31", "pytest-8.0"), "requirements.txt", want); len(got) != 0 {
		t.Errorf("in sync: %+v", got)
	}
	if got := EnvDrift(fstest.MapFS{".venv/pyvenv.cfg": {}}, "requirements.txt", want); len(got) != 0 {
		t.Errorf("unreadable env: %+v", got)
	}
}
