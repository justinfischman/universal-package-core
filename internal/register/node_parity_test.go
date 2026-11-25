package register_test

import (
	"io/fs"
	"testing"

	"github.com/justinfischman/universal-package-core/internal/core"
)

// nodeParityExceptions lists capabilities a Node manager may support at a
// different level than npm, with the reason. It is empty: pnpm and bun match
// npm and yarn on every capability (owner requirement D7.6).
var nodeParityExceptions = map[string]map[core.Capability]string{}

// groupDiagnoser mirrors the optional interface `doctor` uses to check
// dependency groups (internal/cli/groupcheck.go).
type groupDiagnoser interface {
	DiagnoseGroups(fsys fs.FS, groups []string) []core.Finding
}

// optionalInterface reports whether a manager implements one optional
// interface; the probes are type assertions so a removed method flips them.
type optionalInterface struct {
	name string
	has  func(core.PackageManager) bool
}

func implements[T any](pm core.PackageManager) bool {
	_, ok := any(pm).(T)
	return ok
}

// nodeExpectedInterfaces is the fixed set every Node manager, npm included,
// must implement. It is a literal list, not derived from npm, so deleting an
// interface from npm (or any other manager) fails (D7.6). The manager-level
// RunSource, UpProvider and PackageFile interfaces are not part of any manager.
var nodeExpectedInterfaces = []optionalInterface{
	{"group diagnoser (DiagnoseGroups)", implements[groupDiagnoser]},
	{"core.Diagnoser", implements[core.Diagnoser]},
	{"core.DetectRanker", implements[core.DetectRanker]},
	{"core.NativeDryRunner", implements[core.NativeDryRunner]},
	{"core.MemberLister", implements[core.MemberLister]},
}

// nodeInterfaceExceptions lists expected interfaces a manager may lack, with
// the reason. It is empty: npm, yarn, pnpm and bun all implement each one.
var nodeInterfaceExceptions = map[string]map[string]string{}

// TestNodeManagersHaveOptionalInterfaceParity requires every Node manager to
// implement each expected optional interface, so features such as
// dependency-group checks do not exist for only some managers (D7.6).
func TestNodeManagersHaveOptionalInterfaceParity(t *testing.T) {
	regs := defaults(t)
	for _, id := range []string{"npm", "yarn", "pnpm", "bun"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, oi := range nodeExpectedInterfaces {
			if !oi.has(pm) && nodeInterfaceExceptions[id][oi.name] == "" {
				t.Errorf("%s lacks %s with no documented exception", id, oi.name)
			}
		}
	}
}

// nodeExpectedLevels is the fixed level every Node manager, npm included,
// must declare for each capability. It is a literal table, not derived from
// npm, so changing npm's level (or any other manager's) fails (D7.6).
var nodeExpectedLevels = map[core.Capability]core.Level{
	core.CapInit:     core.LevelNative,
	core.CapInstall:  core.LevelNative,
	core.CapAdd:      core.LevelNative,
	core.CapRemove:   core.LevelNative,
	core.CapList:     core.LevelNative,
	core.CapUpdate:   core.LevelNative,
	core.CapOutdated: core.LevelNative,
	core.CapAudit:    core.LevelNative,
	core.CapRun:      core.LevelNative,
	core.CapBuild:    core.LevelFilled,
	core.CapTest:     core.LevelNative,
	core.CapLint:     core.LevelFilled,
	core.CapUp:       core.LevelFilled,
	core.CapDown:     core.LevelFilled,
	core.CapPublish:  core.LevelNative,
}

// TestNodeManagersHaveCapabilityParity requires every Node manager, npm
// included, to declare each capability at the level in nodeExpectedLevels
// (native, filled or unsupported), unless nodeParityExceptions documents why not.
func TestNodeManagersHaveCapabilityParity(t *testing.T) {
	if len(nodeExpectedLevels) != len(core.AllCapabilities) {
		t.Fatalf("nodeExpectedLevels has %d entries, want one per capability (%d)", len(nodeExpectedLevels), len(core.AllCapabilities))
	}
	regs := defaults(t)
	for _, id := range []string{"npm", "yarn", "pnpm", "bun"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, c := range core.AllCapabilities {
			want, got := nodeExpectedLevels[c], pm.Capabilities()[c].Level
			if want == got {
				continue
			}
			if reason := nodeParityExceptions[id][c]; reason == "" {
				t.Errorf("%s declares %s as %v, expected %v, with no documented exception", id, c, got, want)
			}
		}
	}
}
