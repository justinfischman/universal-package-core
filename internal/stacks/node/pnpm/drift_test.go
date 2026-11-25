package pnpm

import (
	"reflect"
	"testing"
	"testing/fstest"
)

func TestLockedVersionsReadsBothLockFormats(t *testing.T) {
	text := `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      is-even:
        specifier: 0.1.0
        version: 0.1.0

packages:

  '@types/node@22.1.0':
    resolution: {integrity: sha512-x}

  is-even@0.1.0:
    resolution: {integrity: sha512-y}

  /is-odd@2.0.0(peer@1.0.0):
    resolution: {integrity: sha512-z}

snapshots:

  is-even@0.1.0: {}
`
	want := map[string][]string{
		"@types/node": {"22.1.0"},
		"is-even":     {"0.1.0"},
		"is-odd":      {"2.0.0"},
	}
	if got := lockedVersions(text); !reflect.DeepEqual(got, want) {
		t.Errorf("locked = %v, want %v", got, want)
	}
}

func TestLockDriftReportsStaleInstall(t *testing.T) {
	lock := "packages:\n\n  is-even@0.1.0:\n    resolution: {}\n"
	files := fstest.MapFS{
		"package.json":                      {Data: []byte(`{"dependencies":{"is-even":"0.1.0"}}`)},
		"pnpm-lock.yaml":                    {Data: []byte(lock)},
		"node_modules/is-even/package.json": {Data: []byte(`{"version":"0.1.2"}`)},
	}
	if findings := lockDrift(files); len(findings) == 0 {
		t.Error("want a drift finding for an installed version the lock does not hold")
	}
	files["node_modules/is-even/package.json"] = &fstest.MapFile{Data: []byte(`{"version":"0.1.0"}`)}
	if findings := lockDrift(files); len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}
