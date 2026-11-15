package poetry

import (
	"strings"
	"testing"
	"testing/fstest"
)

const driftLock = `[[package]]
name = "rich"
version = "13.7.0"
groups = ["main"]

[[package]]
name = "pytest"
version = "8.0"
groups = ["dev"]

[[package]]
name = "colorama"
version = "0.4"
groups = ["main"]
markers = "sys_platform == \"win32\""

[[package]]
name = "old"
version = "1"
category = "main"
`

func TestDiagnoseReportsDriftFromLock(t *testing.T) {
	p := newPoetry(t)
	site := ".venv/lib/python3.12/site-packages/"
	fsys := fstest.MapFS{
		"pyproject.toml":                      {Data: []byte(pyproject)},
		"poetry.lock":                         {Data: []byte(driftLock)},
		".venv/pyvenv.cfg":                    {},
		site + "rich-12.0.dist-info/METADATA": {},
	}
	got := p.Diagnose(fsys)
	if len(got) != 2 || !strings.Contains(got[0].Message, "old not installed") || !strings.Contains(got[1].Message, "rich (12.0 installed, 13.7.0 in poetry.lock)") {
		t.Fatalf("findings = %+v", got)
	}
	delete(fsys, site+"rich-12.0.dist-info/METADATA")
	fsys[site+"rich-13.7.0.dist-info/METADATA"] = &fstest.MapFile{}
	fsys[site+"old-1.dist-info/METADATA"] = &fstest.MapFile{}
	if got := p.Diagnose(fsys); len(got) != 0 {
		t.Errorf("in sync (dev, marker packages not required): %+v", got)
	}
}
