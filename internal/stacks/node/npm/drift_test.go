package npm

import (
	"strings"
	"testing"
	"testing/fstest"
)

const driftLock = `{"packages":{
 "":{"name":"app"},
 "node_modules/left-pad":{"version":"1.3.0"},
 "node_modules/@scope/pkg":{"version":"1.2.0"},
 "node_modules/left-pad/node_modules/other":{"version":"9.0.0"}}}`

func TestDiagnoseReportsDriftFromLock(t *testing.T) {
	n := newNPM(t)
	fsys := fstest.MapFS{
		"package.json":                       {Data: []byte(`{"dependencies":{"@scope/pkg":"^1","left-pad":"^1"},"devDependencies":{"other":"^9"}}`)},
		"package-lock.json":                  {Data: []byte(driftLock)},
		"node_modules/left-pad/package.json": {Data: []byte(`{"version":"1.0.0"}`)},
	}
	got := n.Diagnose(fsys)
	if len(got) != 2 || !strings.Contains(got[0].Message, "@scope/pkg not installed") || !strings.Contains(got[1].Message, "left-pad (1.0.0 installed, 1.3.0 in package-lock.json)") {
		t.Fatalf("findings = %+v", got)
	}
	fsys["node_modules/left-pad/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"1.3.0"}`)}
	fsys["node_modules/@scope/pkg/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"1.2.0"}`)}
	if got := n.Diagnose(fsys); len(got) != 0 {
		t.Errorf("in sync (nested and unlocked entries ignored): %+v", got)
	}
}
