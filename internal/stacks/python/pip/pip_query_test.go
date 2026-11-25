package pip

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/components/execx"
	"github.com/justinfischman/universal-package-core/internal/core"
)

func TestListAndOutdatedParse(t *testing.T) {
	jsonOut := `[{"name":"rich","version":"13.0","latest_version":"14.0"},{"name":"attrs","version":"23.1","latest_version":"23.1"}]`
	res, err := call(t, newMem(venv), core.CapList, core.Request{}, respond(map[string]string{py + " -m pip list --format json": jsonOut}))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 2 || res.Summary != "rich 13.0\nattrs 23.1" {
		t.Errorf("list = %+v", res)
	}
	res, err = call(t, newMem(venv), core.CapOutdated, core.Request{}, respond(map[string]string{py + " -m pip list --format json --outdated": jsonOut}))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Data.([]core.Package); len(got) != 1 || got[0].Name != "rich" || res.Summary != "rich 13.0 -> 14.0" {
		t.Errorf("outdated = %+v", res)
	}
	if _, err := call(t, newMem(venv), core.CapList, core.Request{}, respond(map[string]string{py + " -m pip list --format json": "nope"})); err == nil {
		t.Error("garbled output must be an error")
	}
	res, err = call(t, newMem(venv), core.CapList, core.Request{}, &execx.FakeRunner{})
	if err != nil || res.Summary != "no dependencies" {
		t.Errorf("empty = %+v, %v", res, err)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	m := newMem(with(venv, map[string]string{"requirements.txt": "rich==1\n"}))
	fake := respond(map[string]string{py + " -m pip freeze": "attrs==23.1\n"})
	for _, c := range []core.Capability{core.CapAdd, core.CapRemove, core.CapUpdate} {
		res, err := call(t, m, c, core.Request{Packages: []string{"attrs"}, DryRun: true}, fake)
		if err != nil || !strings.HasPrefix(res.Summary, "dry run, nothing changed: would ") {
			t.Errorf("%s: %+v, %v", c, res, err)
		}
	}
	if m.writes != 0 || m.files[reqPath] != "rich==1\n" {
		t.Errorf("dry run wrote: writes=%d file=%q", m.writes, m.files[reqPath])
	}
}

func TestDetectAndRank(t *testing.T) {
	p := newPIP(t, newMem(nil))
	if !p.Detect(fstest.MapFS{"requirements.txt": {}}) || p.DetectRank(fstest.MapFS{"requirements.txt": {}}) != rankRequirements {
		t.Error("requirements.txt must be detected at the requirements rank")
	}
	if p.Detect(fstest.MapFS{"pyproject.toml": {}}) || p.DetectRank(fstest.MapFS{"pyproject.toml": {}}) != 0 {
		t.Error("pyproject.toml alone is not pip")
	}
}

func TestDiagnoseMissingVenv(t *testing.T) {
	p := newPIP(t, newMem(nil))
	got := p.Diagnose(fstest.MapFS{"requirements.txt": {}})
	if len(got) != 1 || got[0].Fix != core.CapInstall {
		t.Errorf("findings = %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{"requirements.txt": {}, ".venv/pyvenv.cfg": {}}); len(got) != 0 {
		t.Errorf("with venv: %+v", got)
	}
	if got := p.Diagnose(fstest.MapFS{}); len(got) != 0 {
		t.Errorf("empty: %+v", got)
	}
}

func TestToolInstallInfo(t *testing.T) {
	for goos, wantCmd := range map[string]string{"darwin": "brew install python", "linux": "", "windows": ""} {
		tl := tool(goos)
		if got := strings.Join(tl.InstallCmd, " "); got != wantCmd || tl.InstallHint == "" || tl.Binary == "" {
			t.Errorf("%s: %+v", goos, tl)
		}
	}
	if tool("windows").Binary != "python" || tool("linux").Binary != "python3" {
		t.Error("interpreter name per platform")
	}
}
