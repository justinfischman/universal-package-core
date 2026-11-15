package uv

import (
	"strings"
	"testing"
	"testing/fstest"
)

const driftLock = `[[package]]
name = "app"
version = "0.1.0"
source = { virtual = "." }
dependencies = [
    { name = "rich" },
    { name = "colorama", marker = "sys_platform == 'win32'" },
]

[[package]]
name = "rich"
version = "13.7.0"
dependencies = [{ name = "pygments" }]

[[package]]
name = "pygments"
version = "2.17"

[[package]]
name = "colorama"
version = "0.4"
`

func TestDiagnoseReportsDriftFromLock(t *testing.T) {
	u := newUV(t)
	site := ".venv/lib/python3.12/site-packages/"
	fsys := fstest.MapFS{
		"pyproject.toml":                      {Data: []byte("[project]\nname='app'\n")},
		"uv.lock":                             {Data: []byte(driftLock)},
		".venv/pyvenv.cfg":                    {},
		site + "rich-12.0.dist-info/METADATA": {},
	}
	got := u.Diagnose(fsys)
	if len(got) != 2 || !strings.Contains(got[0].Message, "pygments not installed") || !strings.Contains(got[1].Message, "rich (12.0 installed, 13.7.0 in uv.lock)") {
		t.Fatalf("findings = %+v", got)
	}
	delete(fsys, site+"rich-12.0.dist-info/METADATA")
	fsys[site+"rich-13.7.0.dist-info/METADATA"] = &fstest.MapFile{}
	fsys[site+"pygments-2.17.dist-info/METADATA"] = &fstest.MapFile{}
	if got := u.Diagnose(fsys); len(got) != 0 {
		t.Errorf("in sync (marker-only dependency not required): %+v", got)
	}
}
