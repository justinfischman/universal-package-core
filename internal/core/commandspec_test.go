package core

import (
	"errors"
	"reflect"
	"testing"
)

func TestCommandSpecExpand(t *testing.T) {
	group := GroupSpec{Dev: []string{"--dev"}, Named: []string{"--group", "{group}"}}
	add := CommandSpec{Argv: []string{"add", "{group}", "{script}", "{pkg}{constraint}"}, Group: group}
	lock := CommandSpec{Argv: []string{"lock", "{script}", "{pkg}"}, PerPackage: []string{"--upgrade-package"}}
	cd := CommandSpec{Argv: []string{"-C", "{dir}", "add", "{pkg}@{constraint}"}}
	tests := []struct {
		name string
		spec CommandSpec
		vars Vars
		want []string
	}{
		{"plain", add, Vars{Packages: []string{"requests"}}, []string{"add", "requests"}},
		{"constraint is one element", add, Vars{Packages: []string{"requests"}, Constraint: ">=2.3.4,<3"}, []string{"add", "requests>=2.3.4,<3"}},
		{"shell metacharacters stay inside one element", add, Vars{Packages: []string{"x"}, Constraint: ">1,<2"}, []string{"add", "x>1,<2"}},
		{"dev group", add, Vars{Packages: []string{"a", "b"}, GroupName: "dev"}, []string{"add", "--dev", "a", "b"}},
		{"named group and script", add, Vars{Packages: []string{"a"}, GroupName: "docs", Script: []string{"--script", "s.py"}}, []string{"add", "--group", "docs", "--script", "s.py", "a"}},
		{"per-package flag", lock, Vars{Packages: []string{"a", "b"}}, []string{"lock", "--upgrade-package", "a", "--upgrade-package", "b"}},
		{"windows dir stays one element", cd, Vars{Dir: `C:\Users\me\my proj`, Packages: []string{"p"}, Constraint: ">=1"}, []string{"-C", `C:\Users\me\my proj`, "add", "p@>=1"}},
	}
	for _, tt := range tests {
		got, err := tt.spec.Expand(tt.vars)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Expand = %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
}

func TestCommandSpecRejectsConstraintItCannotPlace(t *testing.T) {
	remove := CommandSpec{Argv: []string{"remove", "{pkg}"}}
	if _, err := remove.Expand(Vars{Packages: []string{"a"}, Constraint: ">=1"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestGroupSpecRejectsFlagLikeName(t *testing.T) {
	if _, err := (GroupSpec{Named: []string{"--group", "{group}"}}).Args("--evil"); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestInstallHintsForAndTool(t *testing.T) {
	h := InstallHints{
		Darwin:  InstallHint{Cmd: []string{"brew", "install", "x"}},
		Windows: InstallHint{Cmd: []string{"w"}, Text: "shown"},
		Other:   InstallHint{Cmd: []string{"o"}},
	}
	tool := ToolSpec{Binary: "x", VersionArgs: []string{"--version"}, Install: h}
	if got := tool.Tool("darwin"); got.InstallHint != "brew install x" || got.InstallCmd[0] != "brew" {
		t.Errorf("darwin = %+v", got)
	}
	if got := tool.Tool("windows"); got.InstallHint != "shown" {
		t.Errorf("windows = %+v", got)
	}
	if got := tool.Tool("plan9"); got.InstallHint != "o" {
		t.Errorf("other = %+v", got)
	}
}
