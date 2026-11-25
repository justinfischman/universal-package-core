package bun

import (
	"reflect"
	"testing"
	"testing/fstest"
)

const driftLock = `{
  "lockfileVersion": 1,
  "workspaces": {
    "": { "name": "demo", "dependencies": { "is-even": "0.1.0" } },
    "packages/app": { "name": "app" },
  },
  "packages": {
    "app": ["app@workspace:packages/app"],
    "is-even": ["is-even@0.1.0", "", {}, "sha512-x"],
    "@types/node": ["@types/node@22.1.0", "", {}, "sha512-y"],
    "is-even/is-odd": ["is-odd@2.0.0", "", {}, "sha512-z"],
  }
}
`

func TestLockedVersionsSkipsWorkspaceMembersAndNestedCopies(t *testing.T) {
	want := map[string][]string{
		"@types/node": {"22.1.0"},
		"is-even":     {"0.1.0"},
	}
	if got := lockedVersions(driftLock); !reflect.DeepEqual(got, want) {
		t.Errorf("locked = %v, want %v", got, want)
	}
}

func TestLockDriftIgnoresWorkspaceMembers(t *testing.T) {
	files := fstest.MapFS{
		"package.json":                      {Data: []byte(`{"dependencies":{"is-even":"0.1.0"}}`)},
		"bun.lock":                          {Data: []byte(driftLock)},
		"node_modules/is-even/package.json": {Data: []byte(`{"version":"0.1.0"}`)},
		"node_modules/app/package.json":     {Data: []byte(`{"version":"1.0.0"}`)},
	}
	if findings := lockDrift(files); len(findings) != 0 {
		t.Errorf("findings = %v, want none for a workspace entry", findings)
	}
}

func TestLockDriftWithWorkspaceReportsStaleInstall(t *testing.T) {
	files := fstest.MapFS{
		"package.json":                      {Data: []byte(`{"dependencies":{"is-even":"0.1.0"}}`)},
		"bun.lock":                          {Data: []byte(driftLock)},
		"node_modules/is-even/package.json": {Data: []byte(`{"version":"0.1.2"}`)},
	}
	if findings := lockDrift(files); len(findings) == 0 {
		t.Error("want a drift finding for an installed version the lock does not hold")
	}
}
