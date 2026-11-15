package yarn

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

const classicLock = `# yarn lockfile v1

"@scope/pkg@^1.0.0", "@scope/pkg@^1.1.0":
  version "1.2.0"
  resolved "x"

left-pad@^1.0.0:
  version "1.3.0"
`

const berryLock = `__metadata:
  version: 8

"left-pad@npm:^1.0.0, left-pad@npm:^1.1.0":
  version: 1.3.0
  resolution: "left-pad@npm:1.3.0"
`

func TestLockedVersions(t *testing.T) {
	want := map[string][]string{"@scope/pkg": {"1.2.0"}, "left-pad": {"1.3.0"}}
	if got := lockedVersions(classicLock); !reflect.DeepEqual(got, want) {
		t.Errorf("classic = %v", got)
	}
	if got := lockedVersions(berryLock); !reflect.DeepEqual(got, map[string][]string{"left-pad": {"1.3.0"}}) {
		t.Errorf("berry = %v", got)
	}
}

func TestDiagnoseReportsDriftFromLock(t *testing.T) {
	y := newYarn(t)
	fsys := fstest.MapFS{
		"package.json":                       {Data: []byte(`{"dependencies":{"@scope/pkg":"^1.0.0","left-pad":"^1.0.0"}}`)},
		"yarn.lock":                          {Data: []byte(classicLock)},
		"node_modules/left-pad/package.json": {Data: []byte(`{"version":"1.0.0"}`)},
	}
	got := y.Diagnose(fsys)
	if len(got) != 2 || !strings.Contains(got[0].Message, "@scope/pkg not installed") || !strings.Contains(got[1].Message, "left-pad (1.0.0 installed, 1.3.0 in yarn.lock)") {
		t.Fatalf("findings = %+v", got)
	}
	fsys["node_modules/left-pad/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"1.3.0"}`)}
	fsys["node_modules/@scope/pkg/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"1.2.0"}`)}
	if got := y.Diagnose(fsys); len(got) != 0 {
		t.Errorf("in sync: %+v", got)
	}
	delete(fsys, "node_modules/left-pad/package.json")
	delete(fsys, "node_modules/@scope/pkg/package.json")
	if got := y.Diagnose(fsys); len(got) != 0 {
		t.Errorf("no node_modules is not drift: %+v", got)
	}
}
