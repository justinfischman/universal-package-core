package pip

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestDiagnoseReportsDriftFromRequirements(t *testing.T) {
	p := newPIP(t, newMem(nil))
	fsys := fstest.MapFS{
		"requirements.txt": {Data: []byte("rich==13.7.0\nrequests\n")},
		".venv/lib/python3.12/site-packages/rich-12.0.dist-info/METADATA": {},
	}
	got := p.Diagnose(fsys)
	if len(got) != 2 || !strings.Contains(got[0].Message, "requests not installed") || !strings.Contains(got[1].Message, "rich (12.0 installed") {
		t.Fatalf("findings = %+v", got)
	}
	fsys[".venv/lib/python3.12/site-packages/rich-13.7.0.dist-info/METADATA"] = &fstest.MapFile{}
	delete(fsys, ".venv/lib/python3.12/site-packages/rich-12.0.dist-info/METADATA")
	fsys[".venv/lib/python3.12/site-packages/requests-2.31.dist-info/METADATA"] = &fstest.MapFile{}
	if got := p.Diagnose(fsys); len(got) != 0 {
		t.Errorf("in sync: %+v", got)
	}
}
