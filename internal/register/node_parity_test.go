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

// TestNodeManagersHaveCapabilityParity requires every Node manager to declare
// each capability at the same level as npm (native, filled or unsupported),
// unless nodeParityExceptions documents why not.
func TestNodeManagersHaveCapabilityParity(t *testing.T) {
	regs := defaults(t)
	ref, ok := regs.PackageManagers.Get("npm")
	if !ok {
		t.Fatal("npm must be registered")
	}
	for _, id := range []string{"yarn", "pnpm", "bun"} {
		pm, ok := regs.PackageManagers.Get(id)
		if !ok {
			t.Fatalf("%s must be registered", id)
		}
		for _, c := range core.AllCapabilities {
			want, got := ref.Capabilities()[c].Level, pm.Capabilities()[c].Level
			if want == got {
				continue
			}
			if reason := nodeParityExceptions[id][c]; reason == "" {
				t.Errorf("%s declares %s as %v, npm as %v, with no documented exception", id, c, got, want)
			}
		}
	}
}
