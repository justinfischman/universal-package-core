package workspace

import (
	"reflect"
	"testing"
)

func TestCompareMembersIgnoresOrderSlashesAndDuplicates(t *testing.T) {
	d := CompareMembers([]string{"b", "a/", "a"}, []string{"./a", `b`})
	if !d.Empty() {
		t.Fatalf("diff = %+v", d)
	}
}

func TestCompareMembersReportsBothSides(t *testing.T) {
	d := CompareMembers([]string{"a", "b"}, []string{"b", "c"})
	if !reflect.DeepEqual(d.ConfigOnly, []string{"a"}) || !reflect.DeepEqual(d.NativeOnly, []string{"c"}) {
		t.Fatalf("diff = %+v", d)
	}
	if got := d.String(); got != "in config but not known to the manager: a; known to the manager but not in config: c" {
		t.Errorf("String = %q", got)
	}
}

func TestUpWithDependenciesIsTransitiveAndKeepsOrder(t *testing.T) {
	w := &Workspace{apps: map[string]*App{
		"db":  {Name: "db"},
		"api": {Name: "api", Consumes: []string{"db"}},
		"web": {Name: "web", Consumes: []string{"api"}},
		"doc": {Name: "doc"},
	}}
	r := &fakeRunner{}
	got, err := Orchestrator{Workspace: w, Runner: r}.Up(t.Context(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db", "api", "web"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("started %v, want %v", got, want)
	}
	if _, err := (Orchestrator{Workspace: w, Runner: r}).Up(t.Context(), "ghost"); err == nil {
		t.Error("unknown app must fail")
	}
}
