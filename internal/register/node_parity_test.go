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

var nodeOptionalInterfaces = []optionalInterface{
	{"group diagnoser (DiagnoseGroups)", implements[groupDiagnoser]},
	{"core.Diagnoser", implements[core.Diagnoser]},
	{"core.DetectRanker", implements[core.DetectRanker]},
	{"core.NativeDryRunner", implements[core.NativeDryRunner]},
	{"core.MemberLister", implements[core.MemberLister]},
	{"core.RunSource", implements[core.RunSource]},
	{"core.UpProvider", implements[core.UpProvider]},
	{"core.PackageFile", implements[core.PackageFile]},
}

// nodeInterfaceExceptions lists optional interfaces npm implements that a
// manager may lack, with the reason. It is empty.
var nodeInterfaceExceptions = map[string]map[string]string{}

// TestNodeManagersHaveOptionalInterfaceParity requires every Node manager to
// implement each optional interface npm implements, so features such as
// dependency-group checks do not exist for only some managers (D7.6).
func TestNodeManagersHaveOptionalInterfaceParity(t *testing.T) {
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
		for _, oi := range nodeOptionalInterfaces {
			if oi.has(ref) && !oi.has(pm) && nodeInterfaceExceptions[id][oi.name] == "" {
				t.Errorf("%s lacks %s, which npm implements, with no documented exception", id, oi.name)
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
