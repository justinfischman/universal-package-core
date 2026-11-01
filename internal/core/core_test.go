package core_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/core/coretest"
)

func TestRegistry(t *testing.T) {
	r := core.NewRegistry[core.Language](core.KindLanguage)
	for _, id := range []string{"zeta", "alpha", "mid"} {
		if err := r.Register(coretest.Language{LangID: id, PMs: []string{"x"}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Register(coretest.Language{LangID: "alpha", PMs: []string{"x"}}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("duplicate: got %v", err)
	}
	if err := r.Register(coretest.Language{LangID: "", PMs: []string{"x"}}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("empty id: got %v", err)
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get(missing) = ok")
	}
	got := fmt.Sprint(r.IDs())
	if got != "[alpha mid zeta]" {
		t.Errorf("IDs not sorted: %s", got)
	}
	if n := len(r.List()); n != 3 {
		t.Errorf("List len = %d", n)
	}
}

func TestRegistryRejectsWrongKind(t *testing.T) {
	r := core.NewRegistry[core.ProjectType](core.KindLanguage)
	if err := r.Register(coretest.ProjectType{TypeID: "gadget"}); !errors.Is(err, core.ErrInvalidRequest) {
		t.Errorf("kind mismatch: got %v", err)
	}
}

func TestModuleIDsDeduplicates(t *testing.T) {
	regs, err := coretest.Registries()
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprint(regs.ModuleIDs())
	want := "[alpha alpha-one alpha-two beta beta-one gadget]"
	if got != want {
		t.Errorf("ModuleIDs = %s, want %s", got, want)
	}
}

func TestBasePackageManagerDispatch(t *testing.T) {
	pm, err := coretest.NewPackageManager("fakepm", "fakelang", core.CapLint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pm.Execute(context.Background(), core.CapAdd, core.Request{}); err != nil {
		t.Fatalf("native add: %v", err)
	}
	if len(pm.Calls) != 1 || pm.Calls[0] != core.CapAdd {
		t.Errorf("calls = %v", pm.Calls)
	}
	_, err = pm.Execute(context.Background(), core.CapLint, core.Request{})
	var ue *core.UnsupportedError
	if !errors.As(err, &ue) || !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("lint: want UnsupportedError, got %v", err)
	}
	if ue.PackageManager != "fakepm" || ue.Capability != core.CapLint {
		t.Errorf("error does not name pm and capability: %v", ue)
	}
	if core.ExitCode(err) != core.ExitUnsupported {
		t.Errorf("exit code = %d", core.ExitCode(err))
	}
}

func TestDeclareValidation(t *testing.T) {
	b := core.NewBasePackageManager("p", "l", core.Tool{Binary: "p"})
	noop := func(context.Context, core.Request) (core.Result, error) { return core.Result{}, nil }
	cases := map[string]error{
		"unknown capability":         b.Declare("bogus", core.Native(), noop),
		"native without handler":     b.Declare(core.CapAdd, core.Native(), nil),
		"unsupported with handler":   b.Declare(core.CapAdd, core.Unsupported("why"), noop),
		"unsupported without reason": b.Declare(core.CapAdd, core.Unsupported(""), nil),
		"unknown level":              b.Declare(core.CapAdd, core.Support{Level: "weird"}, noop),
	}
	for name, err := range cases {
		if !errors.Is(err, core.ErrInvalidRequest) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestUndeclaredCapabilityFailsLoudly(t *testing.T) {
	b := core.NewBasePackageManager("p", "l", core.Tool{Binary: "p"})
	if _, err := b.Execute(context.Background(), core.CapAdd, core.Request{}); !errors.Is(err, core.ErrUnsupported) {
		t.Errorf("got %v", err)
	}
	if missing := b.Capabilities().Missing(); len(missing) != len(core.AllCapabilities) {
		t.Errorf("Missing() = %d, want %d", len(missing), len(core.AllCapabilities))
	}
}

func TestDetect(t *testing.T) {
	pm, err := coretest.NewPackageManager("fakepm", "fakelang")
	if err != nil {
		t.Fatal(err)
	}
	if !pm.Detect(fstest.MapFS{"fakepm.marker": {}}) {
		t.Error("marker present but not detected")
	}
	if pm.Detect(fstest.MapFS{}) {
		t.Error("empty dir detected")
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, core.ExitOK},
		{"plain", errors.New("boom"), core.ExitFailure},
		{"invalid", fmt.Errorf("x: %w", core.ErrInvalidRequest), core.ExitUsage},
		{"incompatible", fmt.Errorf("x: %w", core.ErrIncompatible), core.ExitUsage},
		{"unsupported", &core.UnsupportedError{PackageManager: "p", Capability: core.CapAdd}, core.ExitUnsupported},
		{"wrapped unsupported", fmt.Errorf("ctx: %w", &core.UnsupportedError{}), core.ExitUnsupported},
		{"tool missing", &core.ToolMissingError{Tool: "uv"}, core.ExitToolMissing},
	}
	for _, c := range cases {
		if got := core.ExitCode(c.err); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestCapabilityHelpers(t *testing.T) {
	if !core.CapAdd.Valid() || core.Capability("nope").Valid() {
		t.Error("Valid() wrong")
	}
	seen := map[core.Capability]bool{}
	for _, c := range core.AllCapabilities {
		if seen[c] {
			t.Errorf("duplicate capability %s", c)
		}
		seen[c] = true
	}
}
